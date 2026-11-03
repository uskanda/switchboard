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
import { age, elapsed, isDone, isQueued, isRunning, isWaiting, issuePath, plainText, progressText, queueText, timeText } from "@/lib/api";
import { useFlip } from "@/lib/flip";
import { groupByProject, type ProjectGroup } from "@/lib/group";
import { useGroupByProject } from "@/lib/groupPref";
import { useInbox } from "@/lib/inbox";
import type { ViewItem } from "@/lib/pending";
import { cn } from "@/lib/utils";

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
 * column head says what it is; timeWords, for a card, says it in its own words ("5分前から待ち"). A row this tab asked
 * something of (lib/pending.ts) shows a turning ring and slowly pulsing words that it is being processed, said to a
 * screen reader as a status; index.css stops both under prefers-reduced-motion, and the words stay readable.
 */
function cells(it: ViewItem) {
  if (it.pending) {
    const merging = it.pending === "merge";
    return {
      tag: (
        <span role="status" className="inline-flex items-center gap-xs text-label-md font-medium">
          <Spinner />
          <span className="pending-pulse">{merging ? "マージ処理中" : "処理中"}</span>
        </span>
      ),
      line: merging ? "closeされると一覧から消えます" : "実行の開始を待っています",
      time: merging ? age(it.done_since ?? null) : elapsed(it.running_since),
      timeWords: merging ? timeText("done", it.done_since) : timeText("running", it.running_since),
    };
  }
  if (isDone(it)) {
    return {
      tag: it.auto_merge ? (
        <span className="inline-flex flex-wrap items-center gap-xs">
          <span className="text-label-md font-medium">完了</span>
          <AutoMergeBadge />
        </span>
      ) : (
        <span className="text-label-md font-medium">完了</span>
      ),
      line: "closeすると一覧から消えます。コメントすると作業を再開します",
      time: age(it.done_since ?? null),
      timeWords: timeText("done", it.done_since),
    };
  }
  if (isQueued(it)) {
    return {
      tag: <span className="text-label-md font-medium">順番待ち</span>,
      line: queueText(it.reason, it.queued_until),
      time: age(it.queued_since ?? null),
      timeWords: timeText("queued", it.queued_since),
    };
  }
  if (isRunning(it)) {
    return {
      tag: <span className="text-label-md font-medium">実行中</span>,
      line: progressText(it.progress),
      time: elapsed(it.running_since),
      timeWords: timeText("running", it.running_since),
    };
  }
  return {
    tag: <ReasonBadge reason={it.reason} />,
    line: plainText(it.question),
    time: age(it.waiting_since),
    timeWords: timeText("waiting", it.waiting_since),
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
  const c = cells(it);
  return (
    <tr data-flip-key={it.ref}>
      <TableCell className="max-w-0 w-full">
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
          <span className="block truncate text-label-md text-muted">{c.line}</span>
        </Link>
        <ChangeLink it={it} />
      </TableCell>
      <TableCell className="whitespace-nowrap">{c.tag}</TableCell>
      <TableCell className="whitespace-nowrap text-label-md text-muted">{it.host}</TableCell>
      <TableCell className="whitespace-nowrap text-label-md text-muted">{c.time}</TableCell>
    </tr>
  );
}

/**
 * CardList is issues as cards, one under another. A card's frame holds its link to the issue and, below a line, a done
 * issue's change and its merge button, which stay outside that link. Grouped by project, a card leaves out the
 * project's icon, reference and forge's badge, which its group's heading shows, and puts the issue's number before the
 * title, within the title's two lines.
 */
function CardList({ items, selected, grouped }: { items: ViewItem[]; selected?: string; grouped: boolean }) {
  return (
    <ul className="flex flex-col gap-sm">
      {items.map((it) => {
        const c = cells(it);
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
                className="flex flex-col gap-xs rounded-md p-lg focus-visible:outline-none"
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
                <span className="text-label-md text-muted break-words">{c.line}</span>
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
 * badge, then how many issues it has, so that it reads "o/r GitHub 3件".
 */
function GroupHeading({ group }: { group: ProjectGroup }) {
  return (
    <h3 className="flex flex-wrap items-center gap-xs text-label-md text-muted">
      <IssueRef value={group.project} /> <span>{group.items.length}件</span>
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
  const groups = grouped ? groupByProject(items) : null;
  return (
    <>
      <Table className="hidden sm:table lg:hidden">
        <thead>
          <tr>
            <TableHead>issue</TableHead>
            <TableHead className="whitespace-nowrap">状態</TableHead>
            <TableHead className="whitespace-nowrap">端末</TableHead>
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
  return (
    <Button type="button" variant="secondary" aria-pressed={on} onClick={() => onChange(!on)} className={cn(on && "border-primary")}>
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
      プロジェクトでまとめる
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
  const [grouped, setGrouped] = useGroupByProject();
  const root = useRef<HTMLDivElement>(null);
  useFlip(root, view !== null);
  const waiting = view?.filter(isWaiting) ?? [];
  const running = view?.filter(isRunning) ?? [];
  const queued = view?.filter(isQueued) ?? [];
  const done = view?.filter(isDone) ?? [];
  return (
    <div ref={root} className="flex flex-col gap-lg">
      <h2 className="sr-only">一覧</h2>
      <div className="flex flex-wrap justify-end gap-sm">
        <GroupToggle on={grouped} onChange={setGrouped} />
        <NewIssue />
      </div>
      {!inbox && !error && <Loading />}
      {inbox && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">返答待ち</h2>
          {waiting.length > 0 ? (
            <Items items={waiting} timeHead="待ち" selected={selected} grouped={grouped} />
          ) : (
            <p className="text-muted">待ちのissueはありません</p>
          )}
        </section>
      )}
      {running.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">実行中</h2>
          <Items items={running} timeHead="経過" selected={selected} grouped={grouped} />
        </section>
      )}
      {queued.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">順番待ち</h2>
          <Items items={queued} timeHead="順番待ち" selected={selected} grouped={grouped} />
        </section>
      )}
      {done.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">完了（close待ち）</h2>
          <Items items={done} timeHead="完了" selected={selected} grouped={grouped} />
        </section>
      )}
    </div>
  );
}
