import { useEffect, useState } from "react";
import { Link } from "react-router";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Shell } from "@/components/Shell";
import { Alert } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import { Table, TableCell, TableHead } from "@/components/ui/table";
import { ApiError, age, getInbox, issuePath, type Inbox } from "@/lib/api";

function failure(err: unknown): string {
  if (err instanceof ApiError && err.code === "unavailable") {
    return "受信箱のサービス（switchboard inbox serve）に届きません。サービスが動いているかを確かめてください。";
  }
  return `一覧を取れませんでした：${err instanceof Error ? err.message : String(err)}`;
}

export function InboxPage() {
  const [inbox, setInbox] = useState<Inbox | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getInbox().then(setInbox, (err) => setError(failure(err)));
  }, []);

  return (
    <Shell>
      {error && <Alert>{error}</Alert>}
      {inbox?.errors.map((e) => (
        <Alert key={e.forge}>
          {e.forge}を読めませんでした：{e.message}
        </Alert>
      ))}
      {inbox && inbox.items.length === 0 && <p className="text-muted">待ちのissueはありません</p>}
      {inbox && inbox.items.length > 0 && (
        <>
          {/* Wide screens get a table; narrow ones the same items as cards. */}
          <Table className="hidden sm:table">
            <thead>
              <tr>
                <TableHead>issue</TableHead>
                <TableHead>理由</TableHead>
                <TableHead>端末</TableHead>
                <TableHead>待ち</TableHead>
              </tr>
            </thead>
            <tbody>
              {inbox.items.map((it) => (
                <tr key={it.ref}>
                  <TableCell className="max-w-0 w-full">
                    <Link to={issuePath(it.ref)} className="block focus-visible:outline-2 focus-visible:outline-primary">
                      <span className="font-mono text-mono-sm">{it.ref}</span>
                      <span className="block truncate font-medium">{it.title}</span>
                      <span className="block truncate text-label-md text-muted">{it.question}</span>
                    </Link>
                  </TableCell>
                  <TableCell>
                    <ReasonBadge reason={it.reason} />
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-label-md text-muted">{it.host}</TableCell>
                  <TableCell className="whitespace-nowrap text-label-md text-muted">{age(it.waiting_since)}</TableCell>
                </tr>
              ))}
            </tbody>
          </Table>
          <ul className="flex flex-col gap-sm sm:hidden">
            {inbox.items.map((it) => (
              <li key={it.ref}>
                <Link to={issuePath(it.ref)} className="block rounded-md focus-visible:outline-2 focus-visible:outline-primary">
                  <Card className="flex flex-col gap-xs">
                    <span className="font-mono text-mono-sm break-all">{it.ref}</span>
                    <span className="font-medium break-words">{it.title}</span>
                    <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
                      <ReasonBadge reason={it.reason} />
                      <span>{it.host}</span>
                      <span>{age(it.waiting_since)}</span>
                    </span>
                    <span className="text-label-md text-muted break-words">{it.question}</span>
                  </Card>
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </Shell>
  );
}
