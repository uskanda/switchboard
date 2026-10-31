import { Markdown } from "@/components/Markdown";
import { Card } from "@/components/ui/card";
import type { Comment, Detail } from "@/lib/api";
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

function Byline({ author, at, agent }: { author: string; at: string | null | undefined; agent?: boolean }) {
  return (
    <p className="flex flex-wrap items-center gap-sm text-label-md text-muted">
      {agent && <AgentMark />}
      <span className="font-medium">{author || "?"}</span>
      <time dateTime={at ?? undefined}>{when(at)}</time>
    </p>
  );
}

/**
 * shown picks the comments to draw: a claim of a run folds to one line, and of the progress comments only the
 * latest stays. The latest question of the agent is marked to stand out.
 */
function shown(comments: Comment[]) {
  const last = (kind: string) => comments.reduce((at, c, i) => (c.agent && c.kind === kind ? i : at), -1);
  const lastProgress = last("progress");
  const lastQuestion = last("waiting");
  return comments
    .map((c, i) => ({ c, i, folded: c.agent && c.kind === "running", latest: i === lastQuestion }))
    .filter(({ c, i }) => !(c.agent && c.kind === "progress" && i !== lastProgress));
}

/** Thread draws the issue's body and comments, oldest first, as Markdown with raw HTML shown as text. */
export function Thread({ detail }: { detail: Detail }) {
  return (
    <ol className="flex flex-col gap-sm" aria-label="スレッド">
      <li>
        <Card className="flex flex-col gap-sm">
          <Byline author={detail.author ?? ""} at={detail.created_at} />
          {detail.body ? <Markdown text={detail.body} /> : <p className="text-muted">本文はありません</p>}
        </Card>
      </li>
      {shown(detail.comments ?? []).map(({ c, i, folded, latest }) =>
        folded ? (
          <li key={i} className="flex flex-wrap items-center gap-sm px-lg text-label-md text-muted">
            <AgentMark />
            <span>着手しました</span>
            <time dateTime={c.created_at}>{when(c.created_at)}</time>
          </li>
        ) : (
          <li key={i}>
            <Card className={cn("flex flex-col gap-sm", latest && "border-2 border-primary")} aria-current={latest ? "true" : undefined}>
              <Byline author={c.author} at={c.created_at} agent={c.agent} />
              <Markdown text={c.body} />
            </Card>
          </li>
        ),
      )}
    </ol>
  );
}
