import { useEffect, useLayoutEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent, type ReactNode } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { IssueRef } from "@/components/IssueRef";
import { Loading } from "@/components/Loading";
import { AutoMergeBadge, CancelAutoMergeButton, MergeButton, changeName } from "@/components/MergeButton";
import { PlanQuestions } from "@/components/PlanQuestions";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Thread } from "@/components/Thread";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { currentLang, defineMessages, errorText, useLang, useMessages, type Lang } from "@/lib/i18n";
import { common } from "@/lib/i18n/common";
import { ApiError, getIssue, isWaiting, issuePath, postReply, queueText, stateDiffers, timeText, watchKey, type Detail } from "@/lib/api";
import { draftOf, keepDraft } from "@/lib/drafts";
import { useInbox } from "@/lib/inbox";
import { useHeightVar } from "@/lib/height";
import { useMinute } from "@/lib/now";
import { cachedIssue, rememberIssue } from "@/lib/issueCache";
import { composeReply, defaultAnswers, parsePlanQuestions, type Answer } from "@/lib/planQuestions";
import { addedCards, cardsOf, revealAndScroll, targetOf, type Card as ThreadCard } from "@/lib/scrollTarget";
import { markSent } from "@/lib/sent";
import { toast } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { nextWait } from "@/lib/waits";

const messages = defineMessages({
  ja: {
    /** retry is the reply that runs a failed run again: a trusted comment is all a wait on an error needs. */
    retry: "再実行してください。",
    /**
     * conflictRetry is the reply that runs the fix of a conflict again. It names the conflict, so neither the agent
     * nor a person reading the thread takes it for doing the first work over.
     */
    conflictRetry: "baseとの衝突を解消して、もう一度pushしてください。",
    /** ahead says the state shown was read from the forge after the list was, which has yet to catch up. */
    ahead: "フォージから今読んだ状態です。一覧は更新を待っています",
    notWaiting: "このissueは待ちではなくなりました。返答は送っていません。",
    /** cached follows the error of a read when the issue's last read is shown instead. */
    cached: (failure: string) => `${failure} 前回読んだ内容を表示しています。`,
    draftKept: "返答の書きかけは、そのissueに残っています。",
    notMergeable: "このissueは、ここからはマージできません。",
    notMergeableWhy: "マージできるのは、openで完了のissueのうち、PR/MRの番号とコミットが分かるものだけです。",
    planHint: (word: ReactNode) => (
      <>
        この計画で進めるなら「承認する」を押すか、{word} と送ると承認になり、作業が始まります。直してほしいところがあれば、そのまま書いて「返答を送る」を押してください。
      </>
    ),
    questionsHint:
      "問いごとに選んで「この内容で返答」を押します。返答の欄に書いた文は、全体への補足として加わります。返答の欄の文だけを送るなら「返答を送る」を押します。",
    questionsInWordsHint: "問いごとの答えを、返答の欄に書いて送ってください。",
    errorHint:
      "エージェントの実行が失敗しました。同じ作業をやり直すなら「再実行する」を押してください。直し方を伝えるなら、返答の欄に書いて「返答を送る」を押してください。",
    ciHint: "返答するとエージェントが実行を再開し、CIの失敗を自動で直す回数を数え直します。",
    conflictHint:
      "baseとの衝突を自動で直せませんでした。もう一度直させるなら「再実行する」を押してください。直し方を伝えるなら、返答の欄に書いて「返答を送る」を押してください。どちらでも、自動で直す回数を数え直します。",
    pushHint: "pushが拒否されました。返答する前に、エージェントのトークンかリモートの設定（ブランチの保護など）を確かめてください。",
    /** resumed is what a running issue says once its run started on a reply sent from this page. */
    resumed: "返答を受けて、エージェントが作業を再開しました。質問か完了になると、この画面が自動で切り替わります。",
    running: "実行中です。質問か完了になると、この画面が自動で切り替わります。",
    back: "← 一覧へ",
    backLabel: "一覧へ",
    processing: "処理中",
    openForge: "フォージで開く ↗",
    openForgeLabel: "フォージで開く（新しいタブ）",
    reading: "更新を取得中",
    gone: "このissueは、待ちでも実行中でもなくなりました。",
    creating: "作ったばかりのissueです。エージェントが取りかかると、この画面が自動で切り替わります。",
    sent: "送りました。エージェントが作業を再開します。",
    nextLabel: (title: string) => `次の待ちへ：${title}`,
    next: "次の待ちへ",
    noNext: "他に待ちはありません",
    reply: "返答",
    untrusted:
      "あなたはこのプロジェクトの信頼済みのユーザー（trusted_authors）ではありません。返答は投稿されますが、エージェントへの指示としては扱われません。",
    finished: "作業は終わっています。返答すると、エージェントが同じPR/MRで作業を再開します。",
    noLongerWaiting: "このissueは待ちではなくなりました。返答は送れません。",
    preview: "「この内容で返答」で送る文",
    send: "返答を送る",
    sending: "送っています",
    sendAnswers: "この内容で返答",
    approve: (word: string) => `承認する（${word} を送る）`,
    runAgain: "再実行する",
    queuedNoReply: "順番待ちのため、まだ返答は要りません",
    notWaitingNoReply: "このissueは待ちではありません。返答は送れません。",
    noticeLabel: "自動更新の知らせ",
    notice: "自動更新がありました",
  },
  en: {
    retry: "Please run it again.",
    conflictRetry: "Resolve the conflict with the base branch and push again.",
    ahead: "Just read from the forge. The list has not caught up yet.",
    notWaiting: "This issue is no longer waiting. The reply was not sent.",
    cached: (failure) => `${failure} Showing the issue as it was last read.`,
    draftKept: "Your draft reply is still on that issue.",
    notMergeable: "This issue cannot be merged from here.",
    notMergeableWhy: "Only an open, done issue with a known pull or merge request number and commit can be merged here.",
    planHint: (word) => (
      <>
        To go ahead with this plan, press "Approve" or send {word} to approve it and start the work. To ask for changes, write them and press "Send reply".
      </>
    ),
    questionsHint:
      'Choose an answer to each question and press "Send answers". Text in the reply field is added as an overall note. To send only the reply field, press "Send reply".',
    questionsInWordsHint: "Write your answer to each question in the reply field and send it.",
    errorHint:
      'The agent\'s run failed. To do the same work again, press "Run again". To say how to fix it, write it in the reply field and press "Send reply".',
    ciHint: "A reply resumes the agent's run and resets the count of automatic CI fixes.",
    conflictHint:
      'The conflict with the base branch could not be fixed automatically. To have the agent try again, press "Run again". To say how to fix it, write it in the reply field and press "Send reply". Either one resets the count of automatic fixes.',
    pushHint: "The push was rejected. Before replying, check the agent's token or the remote's settings, such as branch protection.",
    resumed: "The agent resumed the work on your reply. This page updates on its own when it asks a question or finishes.",
    running: "Running. This page updates on its own when the agent asks a question or finishes.",
    back: "← Back to inbox",
    backLabel: "Back to inbox",
    processing: "Processing",
    openForge: "Open on the forge ↗",
    openForgeLabel: "Open on the forge (new tab)",
    reading: "Updating",
    gone: "This issue is no longer waiting or running.",
    creating: "This issue was just created. This page updates on its own when the agent starts on it.",
    sent: "Sent. The agent resumes the work.",
    nextLabel: (title) => `Next waiting issue: ${title}`,
    next: "Next waiting issue",
    noNext: "No other issues waiting",
    reply: "Reply",
    untrusted:
      "You are not a trusted user of this project (trusted_authors). Your reply is posted, but the agent does not take it as instructions.",
    finished: "The work is done. A reply makes the agent resume it on the same pull or merge request.",
    noLongerWaiting: "This issue is no longer waiting, so you cannot reply.",
    preview: 'What "Send answers" posts',
    send: "Send reply",
    sending: "Sending",
    sendAnswers: "Send answers",
    approve: (word) => `Approve (sends ${word})`,
    runAgain: "Run again",
    queuedNoReply: "Queued, so no reply is needed yet",
    notWaitingNoReply: "This issue is not waiting, so you cannot reply.",
    noticeLabel: "Update notice",
    notice: "Show new comments",
  },
});

/** retryTextOf is the fixed reply, in the language lang, that the button to run again sends on a wait with this reason. */
function retryTextOf(lang: Lang, reason: string): string {
  return reason === "conflict" ? messages[lang].conflictRetry : messages[lang].retry;
}

function failure(lang: Lang, err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "not_waiting":
        return messages[lang].notWaiting;
      case "unavailable":
        return common[lang].unreachable;
    }
  }
  return common[lang].failed(errorText(err));
}

/** Failure is an error to show, and whether the issue's last read is shown in place of the read that failed. */
type Failure = { err: unknown; cached?: boolean };

/** failureText says what went wrong, in the language shown. */
function failureText(lang: Lang, f: Failure): string {
  return f.cached ? messages[lang].cached(failure(lang, f.err)) : failure(lang, f.err);
}

/**
 * PlanHint says how to answer a wait of the plan stage: the approval word approves a plan, and questions are answered
 * by choosing (answerable) or in words. It never offers the approval word for questions: after a plan, it would
 * approve that plan. For a CI that failed past the fix limit and a refused push, it says what a reply does and what
 * to check first. For a failed run (error) and a conflict past the fix limit (conflict), it offers running again in one
 * click, or a reply that says how to fix it; for a conflict, either counts the fixes over.
 */
function PlanHint({ reason, word, answerable }: { reason: string; word: string; answerable: boolean }) {
  const t = useMessages(messages);
  const code = <code className="font-mono text-mono-sm">{word}</code>;
  const hints: Record<string, ReactNode> = {
    plan: t.planHint(code),
    "plan-questions": answerable ? t.questionsHint : t.questionsInWordsHint,
    error: t.errorHint,
    ci: t.ciHint,
    conflict: t.conflictHint,
    push: t.pushHint,
  };
  return reason in hints ? <p className="text-label-md text-muted">{hints[reason]}</p> : null;
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

/** SentMark tells one wait apart from the next: its start and the agent's latest post of a wait. */
type SentMark = { since: string | null; waitKey: string };

/** markOf reads the mark of the wait detail shows; a back end without the thread gives the agent's post alone. */
function markOf(detail: Detail): SentMark {
  const comments = detail.comments ?? [];
  const last = comments.reduce<(typeof comments)[number] | null>((at, c) => (c.agent && c.kind === "waiting" ? c : at), null);
  return { since: detail.since, waitKey: JSON.stringify(last ? [last.created_at, last.body] : [detail.post]) };
}

/** headerHeight is the height of the page's pinned header, as Header measures it into --header-h. */
const headerHeight = () => parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--header-h")) || 0;

/**
 * IssuePane shows one issue and its reply form. It reads the issue again whenever the issue's row on the list
 * moves (watchKey), keeping what is typed in the reply. What is typed and chosen is kept per issue (lib/drafts.ts),
 * so opening another issue and coming back finds it again. Ctrl+Enter (⌘+Enter) in the reply field sends the field. An issue that left the list stays shown, marked as such.
 * While it waits on the questions of a plan, the questions are answered by choosing: the choices are kept while the
 * same question comment is shown, and start over with the next round's. A wait on a plan is approved, and a wait on a
 * failed run (error) is run again, in one click while the reply is empty.
 * An issue read before in this tab shows its last read at once, marked as being brought up to date until the read
 * arrives; one never read shows that it is loading. While its read shows another state than its row on the list
 * (running, queued, done or waiting; the inbox service reads the list again then), it says the state was read from
 * the forge just now and the list has yet to catch up.
 * Once a reply is sent, it says so while the issue comes back in the wait answered; once the issue runs, queues,
 * waits anew or leaves the list, it shows that state instead, with the reply form again for a new wait. Below the
 * words saying it was sent is the button to the next wait (lib/waits.ts), which takes the focus, or the words that
 * no other wait is left, in which case the focus goes to the words saying it was sent.
 * A waiting or done issue opens scrolled to the card a reply is about (targetOf), once per opening: the cached read
 * drawn first counts, and the read after it moves nothing. A later read that adds cards leaves the place as it is and
 * shows a notice at the bottom instead; pressing it scrolls to the card, and it goes once that card is scrolled into
 * view or another issue opens. Either scroll opens the folded round its card is in first, and another issue starts
 * with its rounds folded again. The way back, the reference, the title and the state stay pinned at the top, and so
 * do the link to a done issue's PR/MR and its merge buttons, so the merge is at hand after reading a long thread; once
 * scrolled, the title is cut to two lines, with the whole of it in its title.
 */
export function IssuePane({ issueRef }: { issueRef: string }) {
  const { inbox, view, markPending } = useInbox();
  const lang = useLang();
  const t = messages[lang];
  // The time in the pinned head moves on each minute.
  useMinute();
  const navigate = useNavigate();
  const item = inbox?.items.find((it) => it.ref === issueRef);
  const key = item ? watchKey(item) : inbox ? "gone" : "first";
  // An issue this tab just created is on the list only as its mark (lib/pending.ts) until the agent takes it up: it
  // has not left the list (issue #174).
  const creating = !item && !!view?.some((it) => it.ref === issueRef && it.pending === "create");
  const gone = inbox !== null && !item && !creating;

  const [detail, setDetail] = useState<Detail | null>(() => cachedIssue(issueRef));
  // reading is set while a read of the issue is on its way; the first one starts as the pane opens.
  const [reading, setReading] = useState(true);
  const [error, setError] = useState<Failure | null>(null);
  const [text, setText] = useState(() => draftOf(issueRef).text);
  // sendingFor is the issue a reply is on its way to. The pane stays the same as another issue opens, so a reply
  // sent before that touches only its own issue (send), and the form is held only while that issue is shown.
  const [sendingFor, setSendingFor] = useState<string | null>(null);
  const sending = sendingFor === issueRef;
  const shown = useRef(issueRef);
  useLayoutEffect(() => {
    shown.current = issueRef;
  }, [issueRef]);
  // sentAt is the wait a reply was sent to, or null when none was.
  const [sentAt, setSentAt] = useState<SentMark | null>(null);
  // sentText is the reply sent to that wait, shown in the thread until the forge's thread has it (issue #182).
  const [sentText, setSentText] = useState<string | null>(null);
  // resumedFor is the issue whose run started on a reply sent from this page, while it runs.
  const [resumedFor, setResumedFor] = useState<string | null>(null);
  const [choices, setChoices] = useState<{ key: string; answers: Answer[] } | null>(() => draftOf(issueRef).choices);
  // fresh is whether detail came from a read of this issue, not from the cache.
  const [fresh, setFresh] = useState(false);
  // notice is the comment index the notice of an update leads to, or null when there is no notice.
  const [notice, setNotice] = useState<number | null>(null);
  // atTop is whether the pane is scrolled to its top, where the title is shown whole.
  const [atTop, setAtTop] = useState(true);

  // A new issue starts with its own draft, empty when there is none. This is done while rendering, so that no frame
  // draws the new issue with the last one's read: the scroll of the opening would go by it.
  const [shownFor, setShownFor] = useState(issueRef);
  if (shownFor !== issueRef) {
    const draft = draftOf(issueRef);
    setShownFor(issueRef);
    setDetail(cachedIssue(issueRef));
    setFresh(false);
    setText(draft.text);
    setSentAt(null);
    setSentText(null);
    setResumedFor(null);
    setError(null);
    setChoices(draft.choices);
    setNotice(null);
  }
  useEffect(() => keepDraft(shownFor, { text, choices }), [shownFor, text, choices]);

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
        setFresh(true);
        setError(null);
        setReading(false);
      },
      (err) => {
        if (stale) return;
        setReading(false);
        setError({ err, cached: !!cachedIssue(issueRef) });
      },
    );
    return () => void (stale = true);
  }, [issueRef, key]);

  const article = useRef<HTMLElement>(null);
  const head = useRef<HTMLDivElement>(null);
  const top = useRef<HTMLDivElement>(null);
  useHeightVar(head, "--pane-head-h", document.documentElement);

  // Another issue starts at its top: below 1024px the page scrolls, from 1024px the pane (issue #172). The scroll to
  // the card a reply is about, below, comes after this one.
  useLayoutEffect(() => {
    window.scrollTo(0, 0);
    const pane = article.current?.closest("main");
    if (pane) pane.scrollTop = 0;
  }, [issueRef]);

  // seen is the issue whose opening scroll is done, whether a read of it has arrived since, and the cards last drawn.
  const seen = useRef<{ ref: string; settled: boolean; cards: ThreadCard[] } | null>(null);
  useLayoutEffect(() => {
    if (!detail) return;
    const was = seen.current;
    const cards = cardsOf(detail.comments ?? []);
    seen.current = { ref: issueRef, settled: fresh, cards };
    if (was?.ref !== issueRef) {
      const target = targetOf(detail);
      if (target !== null) revealAndScroll(article.current, target);
      return;
    }
    // The first read after the cache was shown brings the issue up to date: it is part of the opening.
    if (!was.settled) return;
    const added = addedCards(was.cards, cards);
    if (added.length > 0) setNotice(targetOf(detail) ?? added[0]);
  }, [issueRef, detail, fresh]);

  // The notice goes once its card is scrolled into view; a card already in view when the notice came does not count.
  useEffect(() => {
    const el = notice === null ? null : article.current?.querySelector(`[data-index="${notice}"]`);
    if (!el || typeof IntersectionObserver === "undefined") return;
    let out = false;
    const watch = new IntersectionObserver((entries) => {
      for (const e of entries) {
        if (!e.isIntersecting) out = true;
        else if (out) setNotice(null);
      }
    });
    watch.observe(el);
    return () => watch.disconnect();
  }, [notice]);

  // The pane is at its top while the mark at its top is below the page's header.
  useEffect(() => {
    const el = top.current;
    if (!el || typeof IntersectionObserver === "undefined") return;
    const watch = new IntersectionObserver((entries) => entries.forEach((e) => setAtTop(e.isIntersecting)), {
      rootMargin: `-${Math.round(headerHeight())}px 0px 0px 0px`,
    });
    watch.observe(el);
    return () => watch.disconnect();
  }, []);

  const found = useMemo(() => questionsOf(detail), [detail]);
  // Choices made on these questions; once made, they stay drawn even if the issue stops waiting, like a draft.
  const touched = !!found && choices?.key === found.key;
  const plan = found && ((detail?.waiting && detail.reason === "plan-questions") || touched) ? found : null;
  const answers = plan ? (touched ? choices!.answers : plan.defaults) : [];
  // The reply is written in the language shown, as its preview shows it.
  const composed = plan ? composeReply(lang, plan.parsed.questions, answers, text) : "";
  const word = detail?.approve_word || "/ok";
  // A wait on a plan is approved by its approval word alone, sent in one click while the reply is empty.
  const approving = detail?.reason === "plan" && !plan;
  // A wait on a failed run, or on a conflict past the fix limit, is run again by a fixed reply alone, in the same way.
  // Other failures are not: they need a check or a fix first.
  const retrying = (detail?.reason === "error" || detail?.reason === "conflict") && !plan;

  // justSent is set by a send that worked, so the button to the next wait (or the words saying it was sent, when
  // no other wait is left) takes the focus from the form that goes.
  const justSent = useRef(false);
  const sentLine = useRef<HTMLParagraphElement>(null);
  const nextButton = useRef<HTMLButtonElement>(null);

  async function send(reply: string) {
    const to = issueRef;
    setSendingFor(to);
    setError(null);
    try {
      await postReply(to, reply);
      if (item && isWaiting(item)) markSent(item);
      // The list shows the issue as being processed until the agent takes the reply up (lib/pending.ts).
      markPending(to, "reply");
      if (shown.current === to) {
        justSent.current = true;
        if (detail) {
          setSentAt(markOf(detail));
          setSentText(reply);
        }
        setText("");
        setChoices(null);
      } else {
        // Another issue opened meanwhile: the reply's own issue loses its draft, and the shown one is left alone.
        keepDraft(to, { text: "", choices: null });
      }
    } catch (err) {
      if (shown.current === to) setError({ err });
      else toast.error(failure(currentLang(), err), { description: messages[currentLang()].draftKept });
    } finally {
      setSendingFor((s) => (s === to ? null : s));
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    if (sending || !replyable || text.trim() === "") return;
    void send(text);
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key !== "Enter" || !(e.ctrlKey || e.metaKey)) return;
    e.preventDefault();
    e.currentTarget.form?.requestSubmit();
  }

  const running = detail?.state === "running";
  const queued = detail?.state === "queued";
  // An open done issue takes a reply too: a comment resumes the work until the issue is closed.
  const replyable = !!detail && (detail.waiting || !!detail.done);
  const finished = !!detail?.done && !running && !queued;
  // The issue's own read is newer than its row on the list: until the list catches up, it says which to believe. An
  // issue that left the list says so elsewhere, and one being read again says that instead.
  const ahead = !!item && !!detail && fresh && !reading && stateDiffers(item, detail);
  // A merge waiting for the fix of a conflict starts and ends without the issue changing on the forge, so the detail
  // is not read again: the listing says whether one waits, and the detail only until the first listing arrives.
  const autoMerge = !!(item ? item.auto_merge : !inbox && detail?.auto_merge);
  const now = detail && markOf(detail);
  // The reply stays said to be sent only while the issue is still in the wait (or the done state) it answered.
  const sent = !!sentAt && !!detail && !!now && !gone && replyable && !running && !queued && now.since === sentAt.since && now.waitKey === sentAt.waitKey;
  // ?action=merge (the app's merge button) opens the merge dialog once this issue's own read has arrived; an issue
  // that offers no merge says so instead. Either way the address loses the query, so the dialog opens once.
  const [params, setParams] = useSearchParams();
  const mergeable = finished && !gone && !!detail?.change_number && !!detail?.change_sha;
  const askMerge = params.get("action") === "merge" && fresh && !reading;
  const forgetMerge = () =>
    setParams(
      (p) => {
        p.delete("action");
        return p;
      },
      { replace: true },
    );
  useEffect(() => {
    if (!askMerge || mergeable) return;
    const t = messages[currentLang()];
    toast.error(t.notMergeable, { description: t.notMergeableWhy });
    forgetMerge();
    // forgetMerge changes with every render; the query is what is answered.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [askMerge, mergeable]);
  // Once out of that wait, the mark goes, so coming back to it later does not say sent again.
  useEffect(() => {
    if (!sentAt || sent) return;
    // A reply that took the issue out of its wait into a run is said to have been taken up.
    if (running) setResumedFor(issueRef);
    setSentAt(null);
    setSentText(null);
  }, [sentAt, sent, running, issueRef]);
  // The words of a run taken up last as long as it runs.
  useEffect(() => {
    if (resumedFor && !running) setResumedFor(null);
  }, [resumedFor, running]);
  useEffect(() => {
    if (!sent || !justSent.current) return;
    justSent.current = false;
    (nextButton.current ?? sentLine.current)?.focus();
  }, [sent]);
  // The wait to go to next is looked for while the reply stays said to be sent, on the list as it is now. The listing
  // keeps its order, with current in it; an issue the list shows as being processed (lib/pending.ts) is passed over.
  const busy = new Set(view?.filter((it) => it.pending).map((it) => it.ref));
  const next = sent ? nextWait(inbox?.items.filter((it) => isWaiting(it) && (it.ref === issueRef || !busy.has(it.ref))) ?? [], issueRef) : null;
  // The form stays while something is typed or chosen, so a draft survives the issue leaving its wait.
  const showForm = detail && !sent && (replyable || text !== "" || touched);
  // The reply sent stays in the thread as a card of its own until the forge's thread ends on a comment of a person
  // with the same text (issue #182).
  const lastPerson = [...(detail?.comments ?? [])].reverse().find((c) => !c.agent);
  const pendingReply = sent && sentText !== null && lastPerson?.body.trim() !== sentText.trim() ? sentText : undefined;
  // The list marks the issue as being processed from the reply until the agent takes it up (lib/pending.ts).
  const processing = !!view?.some((it) => it.ref === issueRef && it.pending === "reply");
  // The head shrinks once scrolled; only below 1024px, through the max-lg: classes.
  const compact = !!detail && !atTop;

  return (
    <article ref={article} className={cn("relative flex flex-col gap-lg", !detail && "lg:pt-lg")}>
      <div ref={top} data-pane-top aria-hidden className="pointer-events-none absolute top-0 h-px w-px" />
      <div
        ref={head}
        data-pane-head
        data-compact={compact ? "" : undefined}
        className={cn(
          "sticky top-[var(--header-h,0px)] z-[1] -mx-lg flex flex-col gap-xs border-b border-outline bg-surface px-lg pt-lg pb-md lg:top-0",
          compact && "max-lg:gap-0 max-lg:pt-xs max-lg:pb-xs",
          !detail && "lg:hidden",
        )}
      >
        {/* Below 1024px, once scrolled, the way back shrinks to an arrow beside a one-line title (issue #181). */}
        <div className={cn("flex min-w-0 flex-col gap-xs", compact && "max-lg:flex-row max-lg:items-center max-lg:gap-sm")}>
          <Link
            to="/"
            aria-label={compact ? t.backLabel : undefined}
            className={cn(
              "inline-flex min-h-11 shrink-0 items-center self-start text-label-md text-muted underline lg:hidden",
              compact && "max-lg:min-w-11 max-lg:justify-center max-lg:self-center max-lg:no-underline",
            )}
          >
            {compact ? "←" : t.back}
          </Link>
          {detail && (
            <>
              <IssueRef value={detail.ref} className={cn("self-start", compact && "max-lg:hidden")} />
              <h2
                className={cn("min-w-0 text-headline-md font-semibold break-words", compact && "max-lg:line-clamp-1 lg:line-clamp-2")}
                title={atTop ? undefined : detail.title}
              >
                {detail.title}
              </h2>
            </>
          )}
        </div>
        {detail && (
          <>
            <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
              {processing ? (
                <span className="pending-pulse font-medium">{t.processing}</span>
              ) : running ? (
                <span className="font-medium">{common[lang].state.running}</span>
              ) : queued ? (
                <span className="font-medium">{common[lang].state.queued}</span>
              ) : finished ? (
                <>
                  <span className="font-medium">{common[lang].state.done}</span>
                  {autoMerge && <AutoMergeBadge />}
                </>
              ) : (
                <ReasonBadge reason={detail.reason} />
              )}
              {queued && <span>{queueText(lang, detail.reason)}</span>}
              <span className={cn(compact && "max-lg:hidden")}>{detail.host}</span>
              <span>{timeText(lang, running ? "running" : queued ? "queued" : finished ? "done" : "waiting", detail.since)}</span>
              <a href={detail.url} className={cn("inline-flex min-h-11 items-center underline", compact && "max-lg:hidden")} target="_blank" rel="noreferrer" aria-label={t.openForgeLabel}>
                {t.openForge}
              </a>
              {reading && (
                <span role="status" aria-label={t.reading} className="inline-flex items-center gap-xs">
                  <Spinner />
                  {t.reading}
                </span>
              )}
              {ahead && (
                <span role="status" aria-label={t.ahead}>
                  {t.ahead}
                </span>
              )}
            </span>
            {/* The change and its merge stay in the pinned head, in reach however far the thread is read. */}
            {mergeable && detail.change_number && detail.change_sha && (
              <div className="flex flex-wrap items-center gap-sm">
                {detail.change_url && (
                  <a href={detail.change_url} target="_blank" rel="noreferrer" className="inline-flex min-h-11 min-w-0 items-center break-all font-mono text-mono-sm text-muted underline">
                    {changeName(detail.ref, detail.change_number)}
                  </a>
                )}
                <MergeButton
                  issueRef={detail.ref}
                  number={detail.change_number}
                  sha={detail.change_sha}
                  url={detail.change_url}
                  autoMerge={autoMerge}
                  ask={askMerge}
                  onClosed={() => params.get("action") === "merge" && forgetMerge()}
                />
                {autoMerge && <CancelAutoMergeButton issueRef={detail.ref} number={detail.change_number} />}
              </div>
            )}
          </>
        )}
      </div>
      {error && <Alert>{failureText(lang, error)}</Alert>}
      {!detail && reading && <Loading />}
      {detail && (
        <>
          {gone && <p className="text-muted">{t.gone}</p>}
          {creating && <p className="text-muted">{t.creating}</p>}
          {detail.comments ? (
            <Thread
              key={detail.ref}
              detail={detail}
              pendingReply={pendingReply}
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
            <p ref={sentLine} role="status" tabIndex={-1} className="text-muted outline-none">
              {t.sent}
            </p>
          )}
          {sent &&
            (next ? (
              <Button
                ref={nextButton}
                type="button"
                variant="secondary"
                className="self-start"
                aria-label={t.nextLabel(next.title)}
                onClick={() => navigate(issuePath(next.ref))}
              >
                {t.next}
              </Button>
            ) : (
              <p className="text-muted">{t.noNext}</p>
            ))}
          {showForm && (
            <form onSubmit={submit} className="flex flex-col gap-sm">
              <label htmlFor="reply" className="text-label-md font-medium">
                {t.reply}
              </label>
              <div id="reply-hint">
                {detail.trusted === false && (
                  <p role="note" className="mb-xs rounded-sm bg-attention-container p-sm text-label-md text-attention">
                    {t.untrusted}
                  </p>
                )}
                <PlanHint reason={detail.reason} word={word} answerable={!!plan} />
              </div>
              <Textarea
                id="reply"
                value={text}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={onKeyDown}
                aria-describedby="reply-hint"
                aria-keyshortcuts="Control+Enter Meta+Enter r"
                disabled={sending}
              />
              {finished && <p className="text-label-md text-muted">{t.finished}</p>}
              {!replyable && <p className="text-muted">{t.noLongerWaiting}</p>}
              {plan ? (
                <>
                  <span id="reply-preview" className="text-label-md font-medium text-muted">
                    {t.preview}
                  </span>
                  <section aria-labelledby="reply-preview" className="rounded-md border border-outline bg-surface p-md whitespace-pre-wrap break-words">
                    {composed}
                  </section>
                  <div className="flex flex-wrap justify-end gap-sm">
                    <Button type="submit" variant="secondary" disabled={sending || !replyable || text.trim() === ""}>
                      {t.send}
                    </Button>
                    <Button type="button" disabled={sending || !replyable} onClick={() => void send(composed)}>
                      {sending ? t.sending : t.sendAnswers}
                    </Button>
                  </div>
                </>
              ) : approving ? (
                <div className="flex flex-wrap justify-end gap-sm">
                  <Button type="submit" variant="secondary" disabled={sending || !detail.waiting || text.trim() === ""}>
                    {t.send}
                  </Button>
                  <Button type="button" disabled={sending || !detail.waiting || text.trim() !== ""} onClick={() => void send(word)}>
                    {sending ? t.sending : t.approve(word)}
                  </Button>
                </div>
              ) : retrying ? (
                <div className="flex flex-wrap justify-end gap-sm">
                  <Button type="submit" variant="secondary" disabled={sending || !detail.waiting || text.trim() === ""}>
                    {t.send}
                  </Button>
                  <Button type="button" disabled={sending || !detail.waiting || text.trim() !== ""} onClick={() => void send(retryTextOf(lang, detail.reason))}>
                    {sending ? t.sending : t.runAgain}
                  </Button>
                </div>
              ) : (
                <Button type="submit" disabled={sending || !replyable || text.trim() === ""} className="self-end">
                  {sending ? t.sending : t.send}
                </Button>
              )}
            </form>
          )}
          {running && !sent && !showForm && <p className="text-muted">{resumedFor === issueRef ? t.resumed : t.running}</p>}
          {queued && !replyable && !sent && !showForm && <p className="text-muted">{t.queuedNoReply}</p>}
          {!replyable && !running && !queued && !creating && !sent && !showForm && <p className="text-muted">{t.notWaitingNoReply}</p>}
        </>
      )}
      {notice !== null && (
        <div role="status" aria-label={t.noticeLabel} className="pointer-events-none sticky bottom-lg z-[1] self-center">
          <button
            type="button"
            className="pointer-events-auto min-h-11 rounded-md border border-outline bg-surface-raised px-lg text-label-md font-medium"
            onClick={() => {
              revealAndScroll(article.current, notice);
              setNotice(null);
            }}
          >
            {t.notice}
          </button>
        </div>
      )}
    </article>
  );
}
