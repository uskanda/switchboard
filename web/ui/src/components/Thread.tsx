import type { ReactNode } from "react";
import { Markdown } from "@/components/Markdown";
import { Card } from "@/components/ui/card";
import type { Detail } from "@/lib/api";
import { threadOf, type Round, type Shown } from "@/lib/scrollTarget";
import { cn } from "@/lib/utils";

function when(at: string | null | undefined): string {
  if (!at) return "";
  const d = new Date(at);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString("ja-JP", { dateStyle: "short", timeStyle: "short" });
}

/** AgentMark tells the agent's posts from people's in words; DESIGN.md keeps color for the reasons and errors. */
function AgentMark() {
  return <span className="rounded-sm border border-outline px-xs text-label-md font-medium">エージェント</span>;
}

/**
 * Byline says who wrote a card and when. The agent's posts carry its mark alone: the account it posts with is often
 * the person's own, and naming it beside the mark reads as if that person wrote it.
 */
function Byline({ author, at, agent }: { author: string; at: string | null | undefined; agent?: boolean }) {
  return (
    <p className="flex flex-wrap items-center gap-sm text-label-md text-muted">
      {agent ? <AgentMark /> : <span className="font-medium">{author || "?"}</span>}
      <time dateTime={at ?? undefined}>{when(at)}</time>
    </p>
  );
}

type Answering = { index: number; node: ReactNode };

/** Item draws one comment shown picks: a claim of a run or a queued comment as a line, any other as a card. */
function Item({ item: { c, i, folded, latest }, answering }: { item: Shown; answering?: Answering }) {
  return folded ? (
    <li data-index={i} className="flex flex-wrap items-center gap-sm px-lg text-label-md text-muted">
      <AgentMark />
      <span>{folded === "queued" ? "順番待ちになりました" : "着手しました"}</span>
      <time dateTime={c.created_at}>{when(c.created_at)}</time>
    </li>
  ) : (
    <li data-index={i}>
      <Card className={cn("flex flex-col gap-sm", latest && "border-2 border-primary")} aria-current={latest ? "true" : undefined}>
        <Byline author={c.author} at={c.created_at} agent={c.agent} />
        {answering?.index === i ? answering.node : <Markdown text={c.body} />}
      </Card>
    </li>
  );
}

/**
 * RoundLine draws an answered round folded to one line, like the claim of a run, and its cards once opened. The
 * details is left to keep its own open state: React passes no open, so a read of the same issue leaves it as it is.
 */
function RoundLine({ round: { question, replies }, answering }: { round: Round; answering?: Answering }) {
  return (
    <li>
      <details className="group">
        <summary className="flex min-h-11 cursor-pointer list-none flex-wrap items-center gap-sm rounded-sm px-lg text-label-md text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary [&::-webkit-details-marker]:hidden">
          <AgentMark />
          <span>{`答えた質問・${replies.length > 0 ? `返答${replies.length}件` : "返答なし"}`}</span>
          <time dateTime={question.c.created_at}>{when(question.c.created_at)}</time>
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
 */
export function Thread({ detail, answering }: { detail: Detail; answering?: Answering }) {
  return (
    <ol
      className="flex flex-col gap-sm [&_[data-index]]:scroll-mt-[calc(var(--header-h,0px)+var(--pane-head-h,0px))] lg:[&_[data-index]]:scroll-mt-[var(--pane-head-h,0px)]"
      aria-label="スレッド"
    >
      <li>
        <Card className="flex flex-col gap-sm">
          <Byline author={detail.author ?? ""} at={detail.created_at} />
          {detail.body ? <Markdown text={detail.body} /> : <p className="text-muted">本文はありません</p>}
        </Card>
      </li>
      {threadOf(detail.comments ?? []).map((it) =>
        it.kind === "round" ? (
          <RoundLine key={`round-${it.question.i}`} round={it} answering={answering} />
        ) : (
          <Item key={it.i} item={it} answering={answering} />
        ),
      )}
    </ol>
  );
}
