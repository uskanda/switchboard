import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Link } from "react-router";
import { PlanQuestions } from "@/components/PlanQuestions";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Thread } from "@/components/Thread";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, age, elapsed, getIssue, postReply, queueText, watchKey, type Detail } from "@/lib/api";
import { useInbox } from "@/lib/inbox";
import { cachedIssue, rememberIssue } from "@/lib/issueCache";
import { composeReply, defaultAnswers, parsePlanQuestions, type Answer } from "@/lib/planQuestions";

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
 * PlanHint says how to answer a wait of the plan stage: the approval word approves a plan, and questions are answered
 * by choosing (answerable) or in words. It never offers the approval word for questions: after a plan, it would
 * approve that plan. For a CI that failed past the fix limit and a refused push, it says what a reply does and what
 * to check first.
 */
function PlanHint({ reason, word, answerable }: { reason: string; word: string; answerable: boolean }) {
  const code = <code className="font-mono text-mono-sm">{word}</code>;
  if (reason === "plan") {
    return (
      <p className="text-label-md text-muted">
        この計画で進めるなら「承認する」を押すか、{code} と送ると承認になり、作業が始まります。直してほしいところがあれば、そのまま書いて「返答を送る」を押してください。
      </p>
    );
  }
  if (reason === "plan-questions") {
    return (
      <p className="text-label-md text-muted">
        {answerable
          ? "問いごとに選んで「この内容で返答」を押します。返答の欄に書いた文は、全体への補足として加わります。返答の欄の文だけを送るなら「返答を送る」を押します。"
          : "問いごとの答えを、返答の欄に書いて送ってください。"}
      </p>
    );
  }
  if (reason === "ci") {
    return (
      <p className="text-label-md text-muted">
        返信するとエージェントが実行を再開し、CIの失敗を自動で直す回数を数え直します。
      </p>
    );
  }
  if (reason === "push") {
    return (
      <p className="text-label-md text-muted">
        pushが拒否されました。返信する前に、エージェントのトークンかリモートの設定（ブランチの保護など）を確かめてください。
      </p>
    );
  }
  return null;
}

/** questionsOf reads the question comment of a plan in the agent's latest wait, keyed by the comment itself. */
function questionsOf(detail: Detail | null) {
  const comments = detail?.comments ?? [];
  const index = comments.reduce((at, c, i) => (c.agent && c.kind === "waiting" ? i : at), -1);
  if (!detail || index < 0) return null;
  const parsed = parsePlanQuestions(comments[index].body);
  const c = comments[index];
  return parsed && { index, parsed, key: JSON.stringify([detail.ref, c.created_at, c.body]), defaults: defaultAnswers(parsed.questions) };
}

/**
 * IssuePane shows one issue and its reply form. It reads the issue again whenever the issue's row on the list
 * moves (watchKey), keeping what is typed in the reply. An issue that left the list stays shown, marked as such.
 * While it waits on the questions of a plan, the questions are answered by choosing: the choices are kept while the
 * same question comment is shown, and start over with the next round's.
 * An issue read before in this tab shows its last read at once, marked as being brought up to date until the read
 * arrives; one never read shows that it is loading.
 */
export function IssuePane({ issueRef }: { issueRef: string }) {
  const { inbox } = useInbox();
  const item = inbox?.items.find((it) => it.ref === issueRef);
  const key = item ? watchKey(item) : inbox ? "gone" : "first";
  const gone = inbox !== null && !item;

  const [detail, setDetail] = useState<Detail | null>(() => cachedIssue(issueRef));
  // reading is set while a read of the issue is on its way; the first one starts as the pane opens.
  const [reading, setReading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  const [choices, setChoices] = useState<{ key: string; answers: Answer[] } | null>(null);

  // A new issue starts with an empty reply.
  useEffect(() => {
    setDetail(cachedIssue(issueRef));
    setText("");
    setSent(false);
    setError(null);
    setChoices(null);
  }, [issueRef]);

  const lastKey = useRef(key);
  // readFor is the issue the last read that arrived was of; a read shown from the cache does not count.
  const readFor = useRef<string | null>(null);
  useEffect(() => {
    const prev = lastKey.current;
    lastKey.current = key;
    // An issue that left the list is not read again: what was last shown stays. Nor is one whose first listing just
    // arrived: the read made before it is as new.
    if (readFor.current === issueRef && (key === "gone" || prev === "first")) {
      setReading(false);
      return;
    }
    let stale = false;
    setReading(true);
    getIssue(issueRef).then(
      (d) => {
        if (stale) return;
        readFor.current = issueRef;
        rememberIssue(issueRef, d);
        setDetail(d);
        setError(null);
        setReading(false);
      },
      (err) => {
        if (stale) return;
        setReading(false);
        setError(cachedIssue(issueRef) ? `${failure(err)} 前回読んだ内容を表示しています。` : failure(err));
      },
    );
    return () => void (stale = true);
  }, [issueRef, key]);

  const found = useMemo(() => questionsOf(detail), [detail]);
  // Choices made on these questions; once made, they stay drawn even if the issue stops waiting, like a draft.
  const touched = !!found && choices?.key === found.key;
  const plan = found && ((detail?.waiting && detail.reason === "plan-questions") || touched) ? found : null;
  const answers = plan ? (touched ? choices!.answers : plan.defaults) : [];
  const composed = plan ? composeReply(plan.parsed.questions, answers, text) : "";
  const word = detail?.approve_word || "/ok";
  // A wait on a plan is approved by its approval word alone, sent in one click while the reply is empty.
  const approving = detail?.reason === "plan" && !plan;

  async function send(reply: string) {
    setSending(true);
    setError(null);
    try {
      await postReply(issueRef, reply);
      setSent(true);
      setText("");
      setChoices(null);
    } catch (err) {
      setError(failure(err));
    } finally {
      setSending(false);
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    void send(text);
  }

  const running = detail?.state === "running";
  const queued = detail?.state === "queued";
  // The form stays while something is typed or chosen, so a draft survives the issue leaving its wait.
  const showForm = detail && !sent && (detail.waiting || text !== "" || touched);

  return (
    <article className="flex flex-col gap-lg">
      <Link to="/" className="inline-flex min-h-11 items-center text-label-md text-muted underline lg:hidden">
        ← 一覧へ
      </Link>
      {error && <Alert>{error}</Alert>}
      {!detail && reading && (
        <p role="status" aria-label="読み込んでいます" className="flex items-center gap-sm text-muted">
          <Spinner />
          読み込んでいます
        </p>
      )}
      {detail && (
        <>
          <div className="flex flex-col gap-xs">
            <span className="font-mono text-mono-sm break-all">{detail.ref}</span>
            <h2 className="text-headline-md font-semibold break-words">{detail.title}</h2>
            <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
              {running ? (
                <span className="font-medium">実行中</span>
              ) : queued ? (
                <span className="font-medium">順番待ち</span>
              ) : (
                <ReasonBadge reason={detail.reason} />
              )}
              {queued && <span>{queueText(detail.reason)}</span>}
              <span>{detail.host}</span>
              <span>{running ? elapsed(detail.since) : age(detail.since)}</span>
              <a href={detail.url} className="inline-flex min-h-11 items-center underline" target="_blank" rel="noreferrer">
                フォージで開く（新しいタブ）
              </a>
              {reading && (
                <span role="status" aria-label="更新を取得中" className="inline-flex items-center gap-xs">
                  <Spinner />
                  更新を取得中
                </span>
              )}
            </span>
          </div>
          {gone && <p className="text-muted">このissueは、待ちでも実行中でもなくなりました。</p>}
          {detail.comments ? (
            <Thread
              detail={detail}
              answering={
                plan && !sent
                  ? {
                      index: plan.index,
                      node: <PlanQuestions parsed={plan.parsed} answers={answers} disabled={sending} onChange={(a) => setChoices({ key: plan.key, answers: a })} />,
                    }
                  : undefined
              }
            />
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
              <PlanHint reason={detail.reason} word={word} answerable={!!plan} />
              <Textarea id="reply" value={text} onChange={(e) => setText(e.target.value)} disabled={sending} />
              {!detail.waiting && <p className="text-muted">このissueは待ちではなくなりました。返答は送れません。</p>}
              {plan ? (
                <>
                  <span id="reply-preview" className="text-label-md font-medium text-muted">
                    「この内容で返答」で送る文
                  </span>
                  <section aria-labelledby="reply-preview" className="rounded-md border border-outline bg-surface p-md whitespace-pre-wrap break-words">
                    {composed}
                  </section>
                  <div className="flex flex-wrap justify-end gap-sm">
                    <Button type="submit" variant="secondary" disabled={sending || !detail.waiting || text.trim() === ""}>
                      返答を送る
                    </Button>
                    <Button type="button" disabled={sending || !detail.waiting} onClick={() => void send(composed)}>
                      {sending ? "送っています" : "この内容で返答"}
                    </Button>
                  </div>
                </>
              ) : approving ? (
                <div className="flex flex-wrap justify-end gap-sm">
                  <Button type="submit" variant="secondary" disabled={sending || !detail.waiting || text.trim() === ""}>
                    返答を送る
                  </Button>
                  <Button type="button" disabled={sending || !detail.waiting || text.trim() !== ""} onClick={() => void send(word)}>
                    {sending ? "送っています" : `承認する（${word} を送る）`}
                  </Button>
                </div>
              ) : (
                <Button type="submit" disabled={sending || !detail.waiting || text.trim() === ""} className="self-end">
                  {sending ? "送っています" : "返答を送る"}
                </Button>
              )}
            </form>
          )}
          {running && !sent && !showForm && <p className="text-muted">実行中のため返答できません</p>}
          {queued && !detail.waiting && !sent && !showForm && <p className="text-muted">順番待ちのため、まだ返答は要りません</p>}
          {!detail.waiting && !running && !queued && !sent && !showForm && <p className="text-muted">このissueは待ちではありません。返答は送れません。</p>}
        </>
      )}
    </article>
  );
}
