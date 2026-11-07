import { Badge } from "@/components/ui/badge";
import { isFailure } from "@/lib/api";

/** reasonNames are the short Japanese names of the reasons a wait has, put after the reason's own word. */
const reasonNames: Record<string, string> = {
  question: "質問",
  verify: "確認",
  "plan-questions": "計画の質問",
  plan: "計画の承認",
  error: "実行の失敗",
  permission: "権限の不足",
  push: "pushの失敗",
  ci: "CIの失敗",
  conflict: "衝突",
};

/**
 * ReasonBadge names why an issue waits: the reason's own word, which the documents and logs use, then its Japanese
 * name (issue #178). A reason without a name shows its word alone. The words are always shown; the color only adds.
 */
export function ReasonBadge({ reason }: { reason: string }) {
  const name = reasonNames[reason];
  return (
    <Badge tone={isFailure(reason) ? "error" : "attention"} className="flex-wrap gap-x-xs">
      <span>{reason}</span>
      {name && <span>{name}</span>}
    </Badge>
  );
}
