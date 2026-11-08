import type { ReactNode } from "react";
import { Markdown } from "@/components/Markdown";
import { Card } from "@/components/ui/card";
import { parseProgress, type Detail, type ProgressLine } from "@/lib/api";
import { dateTimeText } from "@/lib/freshness";
import { defineMessages, plural, useLang, useMessages, type Lang } from "@/lib/i18n";
import { threadOf, type Round, type Shown } from "@/lib/scrollTarget";
import { cn } from "@/lib/utils";

const messages = defineMessages({
  ja: {
    agent: "エージェント",
    /** working stands for the words of a progress line that has none yet. */
    working: "作業しています",
    log: (lines: number) => `進捗の記録（${lines}行）`,
    queued: "順番待ちになりました",
    started: "着手しました",
    round: (replies: number) => `答えた質問・${replies > 0 ? `返答${replies}件` : "返答なし"}`,
    thread: "スレッド",
    noBody: "本文はありません",
    you: "あなた",
    pending: "送信済み・フォージへの反映待ち",
  },
  en: {
    agent: "agent",
    working: "Working",
    log: (lines) => `Progress log (${lines} ${plural(lines, "line", "lines")})`,
    queued: "Queued",
    started: "Started",
    round: (replies) => `Answered question, ${replies > 0 ? `${replies} ${plural(replies, "reply", "replies")}` : "no replies"}`,
    thread: "Thread",
    noBody: "No description",
    you: "You",
    pending: "Sent, not yet on the forge",
  },
});

function when(lang: Lang, at: string | null | undefined): string {
  if (!at) return "";
  const d = new Date(at);
  return Number.isNaN(d.getTime()) ? "" : dateTimeText(lang, d);
}

/** AgentMark tells the agent's posts from people's in words; DESIGN.md keeps color for the reasons and errors. */
function AgentMark() {
  const t = useMessages(messages);
  return <span className="rounded-sm border border-outline px-xs text-label-md font-medium">{t.agent}</span>;
}

/**
 * Byline says who wrote a card and when. The agent's posts carry its mark alone: the account it posts with is often
 * the person's own, and naming it beside the mark reads as if that person wrote it.
 */
function Byline({ author, at, agent }: { author: string; at: string | null | undefined; agent?: boolean }) {
  const lang = useLang();
  return (
    <p className="flex flex-wrap items-center gap-sm text-label-md text-muted">
      {agent ? <AgentMark /> : <span className="font-medium">{author || "?"}</span>}
      <time dateTime={at ?? undefined}>{when(lang, at)}</time>
    </p>
  );
}

type Answering = { index: number; node: ReactNode };

/**
 * Progress draws a run's progress comment read line by line (issue #185): the latest line's words as the card's text,
 * with its time, elapsed time and tools in a muted line under them, then every line in a log folded under a details,
 * oldest first. Words not yet said read as working, as on the list.
 */
function Progress({ lines }: { lines: ProgressLine[] }) {
  const t = useMessages(messages);
  const last = lines[lines.length - 1];
  const words = (w: string) => (w && w !== "?" ? w : t.working);
  return (
    <>
      <p className="break-words">{words(last.words)}</p>
      <p className="text-label-md text-muted break-words">{last.head}</p>
      <details className="group">
        <summary className="inline-flex min-h-11 cursor-pointer list-none items-center gap-xs rounded-sm text-label-md text-muted focus-visible:outline-2 focus-visible:outline-primary [&::-webkit-details-marker]:hidden">
          {t.log(lines.length)}
          <span aria-hidden className="transition-transform group-open:rotate-90">
            ▸
          </span>
        </summary>
        <ol className="flex flex-col gap-xs">
          {lines.map((l, k) => (
            <li key={k} className="flex flex-col text-label-md break-words">
              <span className="text-muted">{l.head}</span>
              <span>{words(l.words)}</span>
            </li>
          ))}
        </ol>
      </details>
    </>
  );
}

/** Body draws a comment's text: a progress comment that can be read line by line as Progress, any other as Markdown. */
function Body({ c }: { c: Shown["c"] }) {
  const lines = c.agent && c.kind === "progress" ? parseProgress(c.body) : null;
  return lines ? <Progress lines={lines} /> : <Markdown text={c.body} />;
}

/** Item draws one comment shown picks: a claim of a run or a queued comment as a line, any other as a card. */
function Item({ item: { c, i, folded, latest }, answering }: { item: Shown; answering?: Answering }) {
  const lang = useLang();
  const t = messages[lang];
  return folded ? (
    <li data-index={i} className="flex flex-wrap items-center gap-sm px-lg text-label-md text-muted">
      <AgentMark />
      <span>{folded === "queued" ? t.queued : t.started}</span>
      <time dateTime={c.created_at}>{when(lang, c.created_at)}</time>
    </li>
  ) : (
    <li data-index={i}>
      <Card className={cn("flex flex-col gap-sm", latest && "border-2 border-primary")} aria-current={latest ? "true" : undefined}>
        <Byline author={c.author} at={c.created_at} agent={c.agent} />
        {answering?.index === i ? answering.node : <Body c={c} />}
      </Card>
    </li>
  );
}

/**
 * RoundLine draws an answered round folded to one line, like the claim of a run, and its cards once opened. The
 * details is left to keep its own open state: React passes no open, so a read of the same issue leaves it as it is.
 */
function RoundLine({ round: { question, replies }, answering }: { round: Round; answering?: Answering }) {
  const lang = useLang();
  return (
    <li>
      <details className="group">
        <summary className="flex min-h-11 cursor-pointer list-none flex-wrap items-center gap-sm rounded-sm px-lg text-label-md text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary [&::-webkit-details-marker]:hidden">
          <AgentMark />
          <span>{messages[lang].round(replies.length)}</span>
          <time dateTime={question.c.created_at}>{when(lang, question.c.created_at)}</time>
          <span aria-hidden className="transition-transform group-open:rotate-90">
            ▸
          </span>
        </summary>
        <ol className="mt-sm flex flex-col gap-sm">
          {[question, ...replies].map((s) => (
            <Item key={s.i} item={s} answering={answering} />
          ))}
        </ol>
      </details>
    </li>
  );
}

/**
 * Thread draws the issue's body and comments, oldest first, as Markdown with raw HTML shown as text (see threadOf for
 * what is drawn). Each answered round of the agent's waits is folded to one line that opens on pressing it; the
 * latest wait stays open. answering, when given, draws the comment at its index in its place: the question comment of
 * a plan, drawn for answering. Each comment's card carries the comment's index in data-index, for scrolling to it
 * (revealAndScroll opens a folded round first); it stops below the pinned head of the issue (--header-h and
 * --pane-head-h). A round opened stays open while the Thread is kept; give it the issue as its key, so another issue
 * starts folded.
 * pendingReply, when given, is a reply sent from this page that the forge's thread does not show yet: it is drawn as a
 * last card, said to be sent and waiting to show on the forge (issue #182).
 */
export function Thread({ detail, answering, pendingReply }: { detail: Detail; answering?: Answering; pendingReply?: string }) {
  const t = useMessages(messages);
  return (
    <ol
      className="flex flex-col gap-sm [&_[data-index]]:scroll-mt-[calc(var(--header-h,0px)+var(--pane-head-h,0px))] lg:[&_[data-index]]:scroll-mt-[var(--pane-head-h,0px)]"
      aria-label={t.thread}
    >
      <li>
        <Card className="flex flex-col gap-sm">
          <Byline author={detail.author ?? ""} at={detail.created_at} />
          {detail.body ? <Markdown text={detail.body} /> : <p className="text-muted">{t.noBody}</p>}
        </Card>
      </li>
      {threadOf(detail.comments ?? []).map((it) =>
        it.kind === "round" ? (
          <RoundLine key={`round-${it.question.i}`} round={it} answering={answering} />
        ) : (
          <Item key={it.i} item={it} answering={answering} />
        ),
      )}
      {pendingReply !== undefined && (
        <li>
          <Card className="flex flex-col gap-sm border-dashed">
            <p className="flex flex-wrap items-center gap-sm text-label-md text-muted">
              <span className="font-medium">{t.you}</span>
              <span>{t.pending}</span>
            </p>
            <Markdown text={pendingReply} />
          </Card>
        </li>
      )}
    </ol>
  );
}
