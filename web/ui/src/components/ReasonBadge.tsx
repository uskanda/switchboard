import { Badge } from "@/components/ui/badge";
import { isFailure } from "@/lib/api";
import { defineMessages, useMessages } from "@/lib/i18n";

/**
 * reasonNames are the short names of the reasons a wait has, put after the reason's own word. Japanese names every
 * reason; English names only the reasons whose word alone is unclear, so question, plan-questions and conflict show
 * their word alone.
 */
const reasonNames = defineMessages<Record<string, string>>({
  ja: {
    question: "質問",
    verify: "確認",
    "plan-questions": "計画の質問",
    plan: "計画の承認",
    error: "実行の失敗",
    permission: "権限の不足",
    push: "pushの失敗",
    ci: "CIの失敗",
    conflict: "衝突",
  },
  en: {
    verify: "checks",
    plan: "approval",
    error: "run failed",
    permission: "denied",
    push: "failed",
    ci: "failed",
  },
});

/**
 * ReasonBadge names why an issue waits: the reason's own word, which the documents and logs use, then its short name in
 * the language shown (issue #178). A reason without a name shows its word alone. The words are always shown; the color
 * only adds.
 */
export function ReasonBadge({ reason }: { reason: string }) {
  const name = useMessages(reasonNames)[reason];
  return (
    <Badge tone={isFailure(reason) ? "error" : "attention"} className="flex-wrap gap-x-xs">
      <span>{reason}</span>
      {name && <span>{name}</span>}
    </Badge>
  );
}
