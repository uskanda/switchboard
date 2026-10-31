import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link } from "react-router";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, age, getIssue, postReply, type Detail, type Item } from "@/lib/api";

function failure(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "not_waiting":
        return "このissueは待ちではなくなりました。返答は送っていません。";
      case "unavailable":
        return "受信箱のサービス（switchboard inbox serve）に届きません。";
    }
  }
  return `失敗しました：${err instanceof Error ? err.message : String(err)}`;
}

/**
 * IssuePane is the right pane: one issue's whole post and the reply box. It reads the issue again when the list
 * shows that the issue changed: a new wait, another reason or question, or no longer in the list.
 */
export function IssuePane({ issueRef, item, listed }: { issueRef: string; item?: Item; listed: boolean }) {
  const [detail, setDetail] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);

  // version is what the list says of this issue; null until the list arrives.
  const version = !listed ? null : item ? `${item.waiting_since}|${item.reason}|${item.question}` : "gone";
  const seen = useRef<{ version: string | null } | null>(null);
  const latest = useRef(0);

  useEffect(() => {
    const prev = seen.current;
    seen.current = { version };
    // The first list to arrive tells nothing new about an issue read just before it.
    if (prev && (prev.version === null || prev.version === version)) return;
    const n = ++latest.current;
    getIssue(issueRef).then(
      (d) => {
        if (n !== latest.current) return;
        setDetail(d);
        setError(null);
      },
      (err) => n === latest.current && setError(failure(err)),
    );
  }, [issueRef, version]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSending(true);
    setError(null);
    try {
      await postReply(issueRef, text);
      setSent(true);
      setText("");
    } catch (err) {
      setError(failure(err));
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="flex max-w-[44em] flex-col gap-lg">
      <Link to="/" className="inline-flex min-h-11 items-center self-start text-label-md font-medium md:hidden">
        ← 一覧
      </Link>
      {error && <Alert>{error}</Alert>}
      {detail && (
        <>
          <div className="flex flex-col gap-xs">
            <span className="font-mono text-mono-sm break-all">{detail.ref}</span>
            <h2 className="text-headline-md font-semibold break-words">{detail.title}</h2>
            <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
              <ReasonBadge reason={detail.reason} />
              <span>{detail.host}</span>
              <span>{age(detail.since)}</span>
              <a href={detail.url} className="inline-flex min-h-11 items-center underline" target="_blank" rel="noreferrer">
                フォージで開く（新しいタブ）
              </a>
            </span>
          </div>
          <Card>
            {/* The agent's post, as text: issue text is data, never markup. */}
            <p className="whitespace-pre-wrap break-words">{detail.post}</p>
          </Card>
          {sent && <p className="text-muted">送りました。エージェントが作業を再開します。</p>}
          {detail.waiting && !sent && (
            <form onSubmit={submit} className="flex flex-col gap-sm">
              <label htmlFor="reply" className="text-label-md font-medium">
                返答
              </label>
              <Textarea id="reply" value={text} onChange={(e) => setText(e.target.value)} disabled={sending} />
              <Button type="submit" disabled={sending || text.trim() === ""} className="self-end">
                {sending ? "送っています" : "返答を送る"}
              </Button>
            </form>
          )}
          {!detail.waiting && !sent && <p className="text-muted">このissueは待ちではありません。返答は送れません。</p>}
        </>
      )}
    </div>
  );
}
