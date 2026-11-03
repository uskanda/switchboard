import { Link } from "react-router";
import { IssueRef } from "@/components/IssueRef";
import { Loading } from "@/components/Loading";
import { MergeButton, changeName } from "@/components/MergeButton";
import { NewIssue } from "@/components/NewIssue";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Table, TableCell, TableHead } from "@/components/ui/table";
import { age, elapsed, isDone, isQueued, isRunning, isWaiting, issuePath, plainText, progressText, queueText, type Item } from "@/lib/api";
import { groupByProject, type ProjectGroup } from "@/lib/group";
import { useGroupByProject } from "@/lib/groupPref";
import { useInbox } from "@/lib/inbox";
import { cn } from "@/lib/utils";

/**
 * ChangeLink is the pull or merge request of a done issue, a link of its own beside the issue's, named by its number
 * when the done post gives one, with the button that merges it when the done post names its number and commit. Both stay outside the row's link to the issue.
 */
function ChangeLink({ it }: { it: Item }) {
  if (!isDone(it)) return null;
  const mergeable = !!it.change_number && !!it.change_sha;
  if (!it.change_url && !mergeable) return null;
  return (
    <div className="flex flex-wrap items-center gap-sm">
      {it.change_url && (
        <a href={it.change_url} target="_blank" rel="noreferrer" className="inline-flex min-h-11 min-w-0 items-center break-all font-mono text-mono-sm text-muted underline">
          {it.change_number ? changeName(it.ref, it.change_number) : it.change_url}
        </a>
      )}
      {mergeable && <MergeButton issueRef={it.ref} number={it.change_number!} sha={it.change_sha!} url={it.change_url} />}
    </div>
  );
}

/**
 * What a row shows: a waiting issue its reason, question and wait; a running one its progress and how long it ran; a
 * queued one the limit it waits on and since when; a done one since when it is done.
 */
function cells(it: Item) {
  if (isDone(it)) {
    return {
      tag: <span className="text-label-md font-medium">完了</span>,
      line: "closeすると一覧から消えます。コメントすると作業を再開します",
      time: age(it.done_since ?? null),
    };
  }
  if (isQueued(it)) {
    return {
      tag: <span className="text-label-md font-medium">順番待ち</span>,
      line: queueText(it.reason),
      time: age(it.queued_since ?? null),
    };
  }
  if (isRunning(it)) {
    return {
      tag: <span className="text-label-md font-medium">実行中</span>,
      line: progressText(it.progress),
      time: elapsed(it.running_since),
    };
  }
  return { tag: <ReasonBadge reason={it.reason} />, line: plainText(it.question), time: age(it.waiting_since) };
}

/** Row is one issue as a row of the table. */
function Row({ it, selected }: { it: Item; selected?: string }) {
  const c = cells(it);
  return (
    <tr>
      <TableCell className="max-w-0 w-full">
        <Link
          to={issuePath(it.ref)}
          aria-current={it.ref === selected ? "page" : undefined}
          className="block focus-visible:outline-2 focus-visible:outline-primary"
        >
          <IssueRef value={it.ref} />
          <span className="block truncate font-medium">{it.title}</span>
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

/** CardList is issues as cards, one under another. */
function CardList({ items, selected }: { items: Item[]; selected?: string }) {
  return (
    <ul className="flex flex-col gap-sm">
      {items.map((it) => {
        const c = cells(it);
        const current = it.ref === selected;
        return (
          <li key={it.ref}>
            <Link
              to={issuePath(it.ref)}
              aria-current={current ? "page" : undefined}
              className="block rounded-md focus-visible:outline-2 focus-visible:outline-primary"
            >
              <Card className={cn("flex flex-col gap-xs", current && "border-2 border-primary")}>
                <IssueRef value={it.ref} className="self-start" />
                <span className="font-medium break-words">{it.title}</span>
                <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
                  {c.tag}
                  <span>{it.host}</span>
                  <span>{c.time}</span>
                </span>
                <span className="text-label-md text-muted break-words">{c.line}</span>
              </Card>
            </Link>
            <ChangeLink it={it} />
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
 * under a heading for each project, the projects in the order their first items come.
 */
function Items({ items, timeHead, selected, grouped }: { items: Item[]; timeHead: string; selected?: string; grouped: boolean }) {
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
                <Row key={it.ref} it={it} selected={selected} />
              ))}
            </tbody>
          ))
        ) : (
          <tbody>
            {items.map((it) => (
              <Row key={it.ref} it={it} selected={selected} />
            ))}
          </tbody>
        )}
      </Table>
      <div className="flex flex-col gap-md sm:hidden lg:flex">
        {groups ? (
          groups.map((g) => (
            <div key={g.project} className="flex flex-col gap-xs">
              <GroupHeading group={g} />
              <CardList items={g.items} selected={selected} />
            </div>
          ))
        ) : (
          <CardList items={items} selected={selected} />
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
 * ungroups all four sections at once. Until the first listing arrives it says that it is loading.
 */
export function InboxList({ selected }: { selected?: string }) {
  const { inbox, error } = useInbox();
  const [grouped, setGrouped] = useGroupByProject();
  const waiting = inbox?.items.filter(isWaiting) ?? [];
  const running = inbox?.items.filter(isRunning) ?? [];
  const queued = inbox?.items.filter(isQueued) ?? [];
  const done = inbox?.items.filter(isDone) ?? [];
  return (
    <div className="flex flex-col gap-lg">
      <h2 className="sr-only">一覧</h2>
      <div className="flex flex-wrap justify-end gap-sm">
        <GroupToggle on={grouped} onChange={setGrouped} />
        <NewIssue />
      </div>
      {error && <Alert>{error}</Alert>}
      {inbox?.errors.map((e) => (
        <Alert key={e.forge}>
          {e.forge}を読めませんでした：{e.message}
        </Alert>
      ))}
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
