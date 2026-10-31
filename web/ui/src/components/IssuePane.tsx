import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Thread } from "@/components/Thread";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, age, elapsed, getIssue, postReply, watchKey, type Detail } from "@/lib/api";
import { useInbox } from "@/lib/inbox";

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

/** PlanHint says how to answer a wait of the plan stage: the approval word approves, anything else is a reply. */
function PlanHint({ reason, word }: { reason: string; word: string }) {
  const code = <code className="font-mono text-mono-sm">{word}</code>;
  if (reason === "plan") {
    return (
      <p className="text-label-md text-muted">
        この計画で進めるなら {code} と送ると承認になり、作業が始まります。直してほしいところがあれば、そのまま書いて送ってください。
      </p>
    );
  }
  if (reason === "plan-questions") {
    return (
      <p className="text-label-md text-muted">
        推奨どおりでよければ {code} と送ってください。変えたい問いがあれば「{word} q2はB」のように続けて書きます。
      </p>
    );
  }
  return null;
}

/**
 * IssuePane shows one issue and its reply form. It reads the issue again whenever the issue's row on the list
 * moves (watchKey), keeping what is typed in the reply. An issue that left the list stays shown, marked as such.
 */
export function IssuePane({ issueRef }: { issueRef: string }) {
  const { inbox } = useInbox();
  const item = inbox?.items.find((it) => it.ref === issueRef);
  const key = item ? watchKey(item) : inbox ? "gone" : "first";
  const gone = inbox !== null && !item;

  const [detail, setDetail] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);

  // A new issue starts with an empty reply.
  useEffect(() => {
    setDetail(null);
    setText("");
    setSent(false);
    setError(null);
  }, [issueRef]);

  useEffect(() => {
    // An issue that left the list is not read again: what was last shown stays.
    if (key === "gone" && detail?.ref === issueRef) return;
    let stale = false;
    getIssue(issueRef).then(
      (d) => {
        if (stale) return;
        setDetail(d);
        setError(null);
      },
      (err) => !stale && setError(failure(err)),
    );
    return () => void (stale = true);
    // detail is left out on purpose: reading again follows the list, not the answer itself.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [issueRef, key]);

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

  const running = detail?.state === "running";
  // The form stays while something is typed, so a draft survives the issue leaving its wait.
  const showForm = detail && !sent && (detail.waiting || text !== "");

  return (
    <article className="flex flex-col gap-lg">
      <Link to="/" className="inline-flex min-h-11 items-center text-label-md text-muted underline lg:hidden">
        ← 一覧へ
      </Link>
      {error && <Alert>{error}</Alert>}
      {detail && (
        <>
          <div className="flex flex-col gap-xs">
            <span className="font-mono text-mono-sm break-all">{detail.ref}</span>
            <h2 className="text-headline-md font-semibold break-words">{detail.title}</h2>
            <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
              {running ? <span className="font-medium">実行中</span> : <ReasonBadge reason={detail.reason} />}
              <span>{detail.host}</span>
              <span>{running ? elapsed(detail.since) : age(detail.since)}</span>
              <a href={detail.url} className="inline-flex min-h-11 items-center underline" target="_blank" rel="noreferrer">
                フォージで開く（新しいタブ）
              </a>
            </span>
          </div>
          {gone && <p className="text-muted">このissueは、待ちでも実行中でもなくなりました。</p>}
          {detail.comments ? (
            <Thread detail={detail} />
          ) : (
            <Card>
              {/* A back end without the thread: the agent's post alone, as text. */}
              <p className="whitespace-pre-wrap break-words">{detail.post}</p>
            </Card>
          )}
          {sent && (
            <p role="status" className="text-muted">
              送りました。エージェントが作業を再開します。
            </p>
          )}
          {showForm && (
            <form onSubmit={submit} className="flex flex-col gap-sm">
              <label htmlFor="reply" className="text-label-md font-medium">
                返答
              </label>
              <PlanHint reason={detail.reason} word={detail.approve_word || "/ok"} />
              <Textarea id="reply" value={text} onChange={(e) => setText(e.target.value)} disabled={sending} />
              {!detail.waiting && <p className="text-muted">このissueは待ちではなくなりました。返答は送れません。</p>}
              <Button type="submit" disabled={sending || !detail.waiting || text.trim() === ""} className="self-end">
                {sending ? "送っています" : "返答を送る"}
              </Button>
            </form>
          )}
          {running && !sent && !showForm && <p className="text-muted">実行中のため返答できません</p>}
          {!detail.waiting && !running && !sent && !showForm && <p className="text-muted">このissueは待ちではありません。返答は送れません。</p>}
        </>
      )}
    </article>
  );
}
