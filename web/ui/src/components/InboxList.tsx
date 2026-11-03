import { useRef } from "react";
import { Link } from "react-router";
import { IssueRef } from "@/components/IssueRef";
import { Loading } from "@/components/Loading";
import { MergeButton, changeName } from "@/components/MergeButton";
import { NewIssue } from "@/components/NewIssue";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Card } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Table, TableCell, TableHead } from "@/components/ui/table";
import { age, elapsed, isDone, isQueued, isRunning, isWaiting, issuePath, plainText, progressText, queueText, timeText } from "@/lib/api";
import { useFlip } from "@/lib/flip";
import { useInbox } from "@/lib/inbox";
import type { ViewItem } from "@/lib/pending";
import { cn } from "@/lib/utils";

/**
 * ChangeLink is the pull or merge request of a done issue, a link of its own beside the issue's, named by its number
 * when the done post gives one, with the button that merges it when the done post names its number and commit. Both stay outside the row's link to the issue.
 */
function ChangeLink({ it }: { it: ViewItem }) {
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
      tag: <span className="text-label-md font-medium">完了</span>,
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
 * Items lists issues. On the list's own page, a screen from 640px gets a table; the pane next to an issue (from
 * 1024px) and a narrow screen get the same items as cards. The selected issue is marked.
 */
function Items({ items, timeHead, selected }: { items: ViewItem[]; timeHead: string; selected?: string }) {
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
        <tbody>
          {items.map((it) => {
            const c = cells(it);
            return (
              <tr key={it.ref} data-flip-key={it.ref}>
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
          })}
        </tbody>
      </Table>
      <ul className="flex flex-col gap-sm sm:hidden lg:flex">
        {items.map((it) => {
          const c = cells(it);
          const current = it.ref === selected;
          return (
            <li key={it.ref} data-flip-key={it.ref}>
              <Link
                to={issuePath(it.ref)}
                aria-current={current ? "page" : undefined}
                className="block rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
              >
                <Card className={cn("flex flex-col gap-xs", current && "border-2 border-primary")}>
                  <IssueRef value={it.ref} className="self-start" />
                  <span className="line-clamp-2 font-medium break-words" title={it.title}>
                    {it.title}
                  </span>
                  <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
                    {c.tag}
                    <span>{it.host}</span>
                    <span>{c.timeWords}</span>
                  </span>
                  <span className="text-label-md text-muted break-words">{c.line}</span>
                </Card>
              </Link>
              <ChangeLink it={it} />
            </li>
          );
        })}
      </ul>
    </>
  );
}

/**
 * InboxList is the button for a new issue, then the waiting, the running, the queued and the done issues (open,
 * waiting to be closed), each under its own heading. Until the first listing arrives it says that it is loading. What
 * the listing could not read is not here: InboxErrors shows it above the panes.
 * The rows are the listing with what this tab asked of the issues laid over it (lib/pending.ts). A row that changes
 * places on an update moves there, and a new one fades in (lib/flip.ts).
 */
export function InboxList({ selected }: { selected?: string }) {
  const { inbox, view, error } = useInbox();
  const root = useRef<HTMLDivElement>(null);
  useFlip(root, view !== null);
  const waiting = view?.filter(isWaiting) ?? [];
  const running = view?.filter(isRunning) ?? [];
  const queued = view?.filter(isQueued) ?? [];
  const done = view?.filter(isDone) ?? [];
  return (
    <div ref={root} className="flex flex-col gap-lg">
      <h2 className="sr-only">一覧</h2>
      <NewIssue />
      {!inbox && !error && <Loading />}
      {inbox && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">返答待ち</h2>
          {waiting.length > 0 ? <Items items={waiting} timeHead="待ち" selected={selected} /> : <p className="text-muted">待ちのissueはありません</p>}
        </section>
      )}
      {running.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">実行中</h2>
          <Items items={running} timeHead="経過" selected={selected} />
        </section>
      )}
      {queued.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">順番待ち</h2>
          <Items items={queued} timeHead="順番待ち" selected={selected} />
        </section>
      )}
      {done.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">完了（close待ち）</h2>
          <Items items={done} timeHead="完了" selected={selected} />
        </section>
      )}
    </div>
  );
}
