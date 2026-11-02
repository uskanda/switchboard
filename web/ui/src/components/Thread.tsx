import type { ReactNode } from "react";
import { Markdown } from "@/components/Markdown";
import { Card } from "@/components/ui/card";
import type { Detail } from "@/lib/api";
import { shown } from "@/lib/scrollTarget";
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

/**
 * Thread draws the issue's body and comments, oldest first, as Markdown with raw HTML shown as text (see shown for
 * what is drawn). answering, when given, draws the comment at its index in its place: the question comment of a plan,
 * drawn for answering. Each comment's card carries the comment's index in data-index, for scrolling to it; it stops
 * below the pinned head of the issue (--header-h and --pane-head-h).
 */
export function Thread({ detail, answering }: { detail: Detail; answering?: { index: number; node: ReactNode } }) {
  return (
    <ol
      className="flex flex-col gap-sm [&>li]:scroll-mt-[calc(var(--header-h,0px)+var(--pane-head-h,0px))] lg:[&>li]:scroll-mt-[var(--pane-head-h,0px)]"
      aria-label="スレッド"
    >
      <li>
        <Card className="flex flex-col gap-sm">
          <Byline author={detail.author ?? ""} at={detail.created_at} />
          {detail.body ? <Markdown text={detail.body} /> : <p className="text-muted">本文はありません</p>}
        </Card>
      </li>
      {shown(detail.comments ?? []).map(({ c, i, folded, latest }) =>
        folded ? (
          <li key={i} data-index={i} className="flex flex-wrap items-center gap-sm px-lg text-label-md text-muted">
            <AgentMark />
            <span>{folded === "queued" ? "順番待ちになりました" : "着手しました"}</span>
            <time dateTime={c.created_at}>{when(c.created_at)}</time>
          </li>
        ) : (
          <li key={i} data-index={i}>
            <Card className={cn("flex flex-col gap-sm", latest && "border-2 border-primary")} aria-current={latest ? "true" : undefined}>
              <Byline author={c.author} at={c.created_at} agent={c.agent} />
              {answering?.index === i ? answering.node : <Markdown text={c.body} />}
            </Card>
          </li>
        ),
      )}
    </ol>
  );
}
