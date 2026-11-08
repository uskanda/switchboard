import { useRef } from "react";
import { Link } from "react-router";
import { IssueRef } from "@/components/IssueRef";
import { Loading } from "@/components/Loading";
import { AutoMergeBadge, MergeButton, changeName } from "@/components/MergeButton";
import { NewIssue } from "@/components/NewIssue";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Table, TableCell, TableHead } from "@/components/ui/table";
import { defineMessages, plural, useLang, useMessages, type Lang } from "@/lib/i18n";
import { common } from "@/lib/i18n/common";
import { age, elapsed, isDone, isQueued, isRunning, isWaiting, issuePath, plainText, progressText, queueText, timeText } from "@/lib/api";
import { useFlip } from "@/lib/flip";
import { groupByProject, type ProjectGroup } from "@/lib/group";
import { useGroupByProject } from "@/lib/groupPref";
import { useInbox } from "@/lib/inbox";
import { useMinute } from "@/lib/now";
import type { ViewItem } from "@/lib/pending";
import { cn } from "@/lib/utils";

const messages = defineMessages({
  ja: {
    /** processing and merging tag a row this tab asked something of, with the line under its title. */
    processing: "処理中",
    processingLine: "実行の開始を待っています",
    merging: "マージ処理中",
    mergingLine: "closeされると一覧から消えます",
    /** count is how many issues a project's group has. */
    count: (n: number) => `${n}件`,
    issue: "issue",
    state: "状態",
    host: "端末",
    /** The heads of the time column of the waits and of the runs; the queued and the done say their state. */
    waitingTime: "待ち",
    runningTime: "経過",
    group: "プロジェクトでまとめる",
    /** groupShort is the toggle's words below 640px. */
    groupShort: "まとめる",
    list: "一覧",
    doneHeading: "完了（close待ち）",
    doneNote: "closeすると一覧から消えます。コメントすると作業を再開します",
    noWaits: "待ちのissueはありません",
  },
  en: {
    processing: "Processing",
    processingLine: "Waiting for the run to start",
    merging: "Merging",
    mergingLine: "Leaves the list once closed",
    count: (n) => `${n} ${plural(n, "issue", "issues")}`,
    issue: "Issue",
    state: "Status",
    host: "Host",
    waitingTime: "Waiting",
    runningTime: "Elapsed",
    group: "Group by project",
    groupShort: "Group",
    list: "Inbox",
    doneHeading: "Done (waiting for close)",
    doneNote: "Closed issues leave the list. Comment on one to resume the work.",
    noWaits: "No issues waiting",
  },
});

/**
 * ChangeLink is the pull or merge request of a done issue, a link of its own beside the issue's, named by its number
 * when the done post gives one, with the button that merges it when the done post names its number and commit. Both
 * sit inside the row's cell or card, outside the row's link to the issue.
 */
function ChangeLink({ it, className }: { it: ViewItem; className?: string }) {
  if (!isDone(it)) return null;
  const mergeable = !!it.change_number && !!it.change_sha;
  if (!it.change_url && !mergeable) return null;
  return (
    <div className={cn("flex flex-wrap items-center gap-sm", className)}>
      {it.change_url && (
        <a href={it.change_url} target="_blank" rel="noreferrer" className="inline-flex min-h-11 min-w-0 items-center break-all font-mono text-mono-sm text-muted underline">
          {it.change_number ? changeName(it.ref, it.change_number) : it.change_url}
        </a>
      )}
      {mergeable && <MergeButton issueRef={it.ref} number={it.change_number!} sha={it.change_sha!} url={it.change_url} autoMerge={!!it.auto_merge} />}
    </div>
  );
}

/**
 * What a row shows: a waiting issue its reason, question and wait; a running one what it is doing and how long it ran;
 * a queued one the limit it waits on and since when; a done one since when it is done. time is for the table, whose
 * column head says what it is; timeWords, for a card, says it in its own words ("5分前から待ち", "waiting 5m"). A row
 * this tab asked something of (lib/pending.ts) shows a turning ring and slowly pulsing words that it is being
 * processed, said to a screen reader as a status; index.css stops both under prefers-reduced-motion, and the words stay
 * readable.
 */
function cells(it: ViewItem, lang: Lang) {
  const t = messages[lang];
  const { state } = common[lang];
  if (it.pending) {
    const merging = it.pending === "merge";
    return {
      tag: (
        <span role="status" className="inline-flex items-center gap-xs text-label-md font-medium">
          <Spinner />
          <span className="pending-pulse">{merging ? t.merging : t.processing}</span>
        </span>
      ),
      line: merging ? t.mergingLine : t.processingLine,
      time: merging ? age(lang, it.done_since ?? null) : elapsed(lang, it.running_since),
      timeWords: merging ? timeText(lang, "done", it.done_since) : timeText(lang, "running", it.running_since),
    };
  }
  if (isDone(it)) {
    return {
      tag: it.auto_merge ? (
        <span className="inline-flex flex-wrap items-center gap-xs">
          <span className="text-label-md font-medium">{state.done}</span>
          <AutoMergeBadge />
        </span>
      ) : (
        <span className="text-label-md font-medium">{state.done}</span>
      ),
      // What closing and commenting do is said once, under the heading of the section.
      line: "",
      time: age(lang, it.done_since ?? null),
      timeWords: timeText(lang, "done", it.done_since),
    };
  }
  if (isQueued(it)) {
    return {
      tag: <span className="text-label-md font-medium">{state.queued}</span>,
      line: queueText(lang, it.reason, it.queued_until),
      time: age(lang, it.queued_since ?? null),
      timeWords: timeText(lang, "queued", it.queued_since),
    };
  }
  if (isRunning(it)) {
    return {
      tag: <span className="text-label-md font-medium">{state.running}</span>,
      line: progressText(lang, it.progress),
      time: elapsed(lang, it.running_since),
      timeWords: timeText(lang, "running", it.running_since),
    };
  }
  return {
    tag: <ReasonBadge reason={it.reason} />,
    line: plainText(it.question),
    time: age(lang, it.waiting_since),
    timeWords: timeText(lang, "waiting", it.waiting_since),
  };
}

/**
 * IssueNumber is an issue's number put before its title in a row grouped by project, whose heading already names the
 * project and forge: "#12 Title".
 */
function IssueNumber({ it }: { it: ViewItem }) {
  return (
    <>
      <span className="font-mono text-mono-sm font-normal text-muted">#{it.number}</span>{" "}
    </>
  );
}

/**
 * Row is one issue as a row of the table. Grouped by project, it leaves out the project's icon, reference and forge's
 * badge, which its group's heading shows, and puts the issue's number before the title.
 */
function Row({ it, selected, grouped }: { it: ViewItem; selected?: string; grouped: boolean }) {
  const lang = useLang();
  const c = cells(it, lang);
  return (
    <tr data-flip-key={it.ref}>
      <TableCell>
        <Link
          to={issuePath(it.ref)}
          aria-current={it.ref === selected ? "page" : undefined}
          className="block focus-visible:outline-2 focus-visible:outline-primary"
        >
          {!grouped && <IssueRef value={it.ref} />}
          <span className="block truncate font-medium">
            {grouped && <IssueNumber it={it} />}
            {it.title}
          </span>
          {c.line && <span className="block truncate text-label-md text-muted">{c.line}</span>}
        </Link>
        <ChangeLink it={it} />
      </TableCell>
      <TableCell>{c.tag}</TableCell>
      <TableCell className="truncate text-label-md text-muted" title={it.host}>
        {it.host}
      </TableCell>
      <TableCell className="whitespace-nowrap text-label-md text-muted">{c.time}</TableCell>
    </tr>
  );
}

/**
 * CardList is issues as cards, one under another. A card's frame holds its link to the issue and, below a line, a done
 * issue's change and its merge button, which stay outside that link. Grouped by project, a card leaves out the
 * project's icon, reference and forge's badge, which its group's heading shows, and puts the issue's number before the
 * title, within the title's two lines. With no line for the project, the title would start lower than the card's
 * last line ends above the bottom, its line height adding to the padding, so the padding above it is md, not lg.
 */
function CardList({ items, selected, grouped }: { items: ViewItem[]; selected?: string; grouped: boolean }) {
  const lang = useLang();
  return (
    <ul className="flex flex-col gap-sm">
      {items.map((it) => {
        const c = cells(it, lang);
        const current = it.ref === selected;
        return (
          <li key={it.ref} data-flip-key={it.ref}>
            <Card
              className={cn(
                "flex flex-col p-0 has-[>a:focus-visible]:outline-2 has-[>a:focus-visible]:outline-offset-2 has-[>a:focus-visible]:outline-primary",
                current && "border-2 border-primary",
              )}
            >
              <Link
                to={issuePath(it.ref)}
                aria-current={current ? "page" : undefined}
                className={cn("flex flex-col gap-xs rounded-md focus-visible:outline-none", grouped ? "px-lg pt-md pb-lg" : "p-lg")}
              >
                {!grouped && <IssueRef value={it.ref} className="self-start" />}
                <span className="line-clamp-2 font-medium break-words" title={it.title}>
                  {grouped && <IssueNumber it={it} />}
                  {it.title}
                </span>
                <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
                  {c.tag}
                  <span>{it.host}</span>
                  <span>{c.timeWords}</span>
                </span>
                {c.line && <span className="text-label-md text-muted break-words">{c.line}</span>}
              </Link>
              <ChangeLink it={it} className="border-t border-outline px-lg pt-sm pb-lg" />
            </Card>
          </li>
        );
      })}
    </ul>
  );
}

/**
 * GroupHeading heads a project's group, a step under the section's heading: the project's reference and forge's
 * badge, then how many issues it has, so that it reads "o/r GitHub 3件" ("o/r GitHub 3 issues").
 */
function GroupHeading({ group }: { group: ProjectGroup }) {
  const t = useMessages(messages);
  return (
    <h3 className="flex flex-wrap items-center gap-xs text-label-md text-muted">
      <IssueRef value={group.project} /> <span>{t.count(group.items.length)}</span>
    </h3>
  );
}

/**
 * Items lists issues. On the list's own page, a screen from 640px gets a table; the pane next to an issue (from
 * 1024px) and a narrow screen get the same items as cards. The selected issue is marked. Grouped, the items come
 * under a heading for each project, the projects in the order their first items come, and each row shows the issue's
 * number in place of its reference.
 */
function Items({ items, timeHead, selected, grouped }: { items: ViewItem[]; timeHead: string; selected?: string; grouped: boolean }) {
  const t = useMessages(messages);
  const groups = grouped ? groupByProject(items) : null;
  return (
    <>
      {/* Each section is a table of its own: fixed widths keep the columns of every section in line (issue #179). */}
      <Table className="hidden table-fixed sm:table lg:hidden">
        <colgroup>
          <col />
          <col className="w-[10rem]" />
          <col className="w-[7rem]" />
          <col className="w-[6rem]" />
        </colgroup>
        <thead>
          <tr>
            <TableHead>{t.issue}</TableHead>
            <TableHead className="whitespace-nowrap">{t.state}</TableHead>
            <TableHead className="whitespace-nowrap">{t.host}</TableHead>
            <TableHead className="whitespace-nowrap">{timeHead}</TableHead>
          </tr>
        </thead>
        {groups ? (
          groups.map((g) => (
            <tbody key={g.project}>
              <tr>
                <TableCell colSpan={4} className="pt-md">
                  <GroupHeading group={g} />
                </TableCell>
              </tr>
              {g.items.map((it) => (
                <Row key={it.ref} it={it} selected={selected} grouped />
              ))}
            </tbody>
          ))
        ) : (
          <tbody>
            {items.map((it) => (
              <Row key={it.ref} it={it} selected={selected} grouped={false} />
            ))}
          </tbody>
        )}
      </Table>
      <div className="flex flex-col gap-md sm:hidden lg:flex">
        {groups ? (
          groups.map((g) => (
            <div key={g.project} className="flex flex-col gap-xs">
              <GroupHeading group={g} />
              <CardList items={g.items} selected={selected} grouped />
            </div>
          ))
        ) : (
          <CardList items={items} selected={selected} grouped={false} />
        )}
      </div>
    </>
  );
}

/**
 * GroupToggle turns the grouping by project on and off for the whole list. It says which with aria-pressed and, for
 * the eye, with a check mark and a primary border while it is on.
 */
function GroupToggle({ on, onChange }: { on: boolean; onChange: (on: boolean) => void }) {
  const t = useMessages(messages);
  return (
    <Button
      type="button"
      variant="secondary"
      aria-pressed={on}
      aria-label={t.group}
      onClick={() => onChange(!on)}
      className={cn(on && "border-primary")}
    >
      <svg
        viewBox="0 0 20 20"
        width="18"
        height="18"
        aria-hidden="true"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
        className={cn("shrink-0", !on && "invisible")}
      >
        <path d="M4.5 10.5l3.5 3.5 7.5-8" />
      </svg>
      {/* Below 640px the words shrink, so the buttons fit beside the heading of the waits (issue #184). */}
      <span className="sm:hidden">{t.groupShort}</span>
      <span className="max-sm:hidden">{t.group}</span>
    </Button>
  );
}

/**
 * InboxList is the toggle of the grouping by project and the button for a new issue, then the waiting, the running,
 * the queued and the done issues (open, waiting to be closed), each under its own heading. The toggle groups or
 * ungroups all four sections at once. Until the first listing arrives it says that it is loading. What the listing
 * could not read is not here: InboxErrors shows it above the panes.
 * The rows are the listing with what this tab asked of the issues laid over it (lib/pending.ts). A row that changes
 * places on an update moves there, and a new one fades in (lib/flip.ts).
 */
export function InboxList({ selected }: { selected?: string }) {
  const { inbox, view, error } = useInbox();
  const t = useMessages(messages);
  const { state } = useMessages(common);
  // The times of the rows move on each minute, with or without a new listing.
  useMinute();
  const [grouped, setGrouped] = useGroupByProject();
  const root = useRef<HTMLDivElement>(null);
  useFlip(root, view !== null);
  const waiting = view?.filter(isWaiting) ?? [];
  const running = view?.filter(isRunning) ?? [];
  const queued = view?.filter(isQueued) ?? [];
  const done = view?.filter(isDone) ?? [];
  return (
    <div ref={root} className="flex flex-col gap-lg">
      <h2 className="sr-only">{t.list}</h2>
      {/* The buttons share the row of the heading of the waits, so the waits start right under it (issue #184). */}
      <section className="flex flex-col gap-sm">
        <div className="flex flex-wrap items-center justify-between gap-sm">
          <h2 className="text-label-md font-semibold text-muted">{state.waiting}</h2>
          <div className="flex flex-wrap justify-end gap-sm">
            <GroupToggle on={grouped} onChange={setGrouped} />
            <NewIssue />
          </div>
        </div>
        {!inbox && !error && <Loading />}
        {inbox &&
          (waiting.length > 0 ? (
            <Items items={waiting} timeHead={t.waitingTime} selected={selected} grouped={grouped} />
          ) : (
            <p className="text-muted">{t.noWaits}</p>
          ))}
      </section>
      {running.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">{state.running}</h2>
          <Items items={running} timeHead={t.runningTime} selected={selected} grouped={grouped} />
        </section>
      )}
      {queued.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">{state.queued}</h2>
          <Items items={queued} timeHead={state.queued} selected={selected} grouped={grouped} />
        </section>
      )}
      {done.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">{t.doneHeading}</h2>
          <p className="text-label-md text-muted">{t.doneNote}</p>
          <Items items={done} timeHead={state.done} selected={selected} grouped={grouped} />
        </section>
      )}
    </div>
  );
}
