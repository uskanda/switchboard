import { useEffect, useState } from "react";
import { Link } from "react-router";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Shell } from "@/components/Shell";
import { Alert } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import { Table, TableCell, TableHead } from "@/components/ui/table";
import { ApiError, age, elapsed, getInbox, isRunning, issuePath, type Inbox, type Item } from "@/lib/api";

function failure(err: unknown): string {
  if (err instanceof ApiError && err.code === "unavailable") {
    return "受信箱のサービス（switchboard inbox serve）に届きません。サービスが動いているかを確かめてください。";
  }
  return `一覧を取れませんでした：${err instanceof Error ? err.message : String(err)}`;
}

/** What a row shows: a waiting issue its reason, question and wait; a running one its progress and how long it ran. */
function cells(it: Item) {
  if (isRunning(it)) {
    return {
      tag: <span className="text-label-md font-medium">実行中</span>,
      line: it.progress ?? "?",
      time: elapsed(it.running_since),
    };
  }
  return { tag: <ReasonBadge reason={it.reason} />, line: it.question, time: age(it.waiting_since) };
}

/** Items lists issues: wide screens get a table, narrow ones the same items as cards. */
function Items({ items, timeHead }: { items: Item[]; timeHead: string }) {
  return (
    <>
      <Table className="hidden sm:table">
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
              <tr key={it.ref}>
                <TableCell className="max-w-0 w-full">
                  <Link to={issuePath(it.ref)} className="block focus-visible:outline-2 focus-visible:outline-primary">
                    <span className="font-mono text-mono-sm">{it.ref}</span>
                    <span className="block truncate font-medium">{it.title}</span>
                    <span className="block truncate text-label-md text-muted">{c.line}</span>
                  </Link>
                </TableCell>
                <TableCell className="whitespace-nowrap">{c.tag}</TableCell>
                <TableCell className="whitespace-nowrap text-label-md text-muted">{it.host}</TableCell>
                <TableCell className="whitespace-nowrap text-label-md text-muted">{c.time}</TableCell>
              </tr>
            );
          })}
        </tbody>
      </Table>
      <ul className="flex flex-col gap-sm sm:hidden">
        {items.map((it) => {
          const c = cells(it);
          return (
            <li key={it.ref}>
              <Link to={issuePath(it.ref)} className="block rounded-md focus-visible:outline-2 focus-visible:outline-primary">
                <Card className="flex flex-col gap-xs">
                  <span className="font-mono text-mono-sm break-all">{it.ref}</span>
                  <span className="font-medium break-words">{it.title}</span>
                  <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
                    {c.tag}
                    <span>{it.host}</span>
                    <span>{c.time}</span>
                  </span>
                  <span className="text-label-md text-muted break-words">{c.line}</span>
                </Card>
              </Link>
            </li>
          );
        })}
      </ul>
    </>
  );
}

export function InboxPage() {
  const [inbox, setInbox] = useState<Inbox | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getInbox().then(setInbox, (err) => setError(failure(err)));
  }, []);

  const waiting = inbox?.items.filter((it) => !isRunning(it)) ?? [];
  const running = inbox?.items.filter(isRunning) ?? [];

  return (
    <Shell home>
      {error && <Alert>{error}</Alert>}
      {inbox?.errors.map((e) => (
        <Alert key={e.forge}>
          {e.forge}を読めませんでした：{e.message}
        </Alert>
      ))}
      {inbox && waiting.length === 0 && <p className="text-muted">待ちのissueはありません</p>}
      {waiting.length > 0 && <Items items={waiting} timeHead="待ち" />}
      {running.length > 0 && (
        <section className="flex flex-col gap-sm">
          <h2 className="text-label-md font-semibold text-muted">実行中</h2>
          <Items items={running} timeHead="経過" />
        </section>
      )}
    </Shell>
  );
}
