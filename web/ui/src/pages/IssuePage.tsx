import { useEffect, useState, type FormEvent } from "react";
import { useParams } from "react-router";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Shell } from "@/components/Shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, age, elapsed, getIssue, postReply, type Detail } from "@/lib/api";

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

export function IssuePage() {
  const ref = useParams().ref ?? "";
  const [detail, setDetail] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);

  useEffect(() => {
    getIssue(ref).then(setDetail, (err) => setError(failure(err)));
  }, [ref]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSending(true);
    setError(null);
    try {
      await postReply(ref, text);
      setSent(true);
      setText("");
    } catch (err) {
      setError(failure(err));
    } finally {
      setSending(false);
    }
  }

  const running = detail?.state === "running";

  return (
    <Shell>
      {error && <Alert>{error}</Alert>}
      {detail && (
        <>
          <div className="flex flex-col gap-xs">
            <span className="font-mono text-mono-sm break-all">{detail.ref}</span>
            <h1 className="text-headline-md font-semibold break-words">{detail.title}</h1>
            <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
              {running ? <span className="font-medium">実行中</span> : <ReasonBadge reason={detail.reason} />}
              <span>{detail.host}</span>
              <span>{running ? elapsed(detail.since) : age(detail.since)}</span>
              <a href={detail.url} className="inline-flex min-h-11 items-center underline" target="_blank" rel="noreferrer">
                フォージで開く（新しいタブ）
              </a>
            </span>
          </div>
          <Card>
            {/* The agent's post (while the issue runs, its progress comment), as text: issue text is data, never markup. */}
            <p className="whitespace-pre-wrap break-words">{detail.post}</p>
          </Card>
          {sent && (
            <p role="status" className="text-muted">
              送りました。エージェントが作業を再開します。
            </p>
          )}
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
          {running && !sent && <p className="text-muted">実行中のため返答できません</p>}
          {!detail.waiting && !running && !sent && <p className="text-muted">このissueは待ちではありません。返答は送れません。</p>}
        </>
      )}
    </Shell>
  );
}
