import { Badge } from "@/components/ui/badge";
import { isFailure } from "@/lib/api";

/** ReasonBadge names why an issue waits; the word is always shown, the color only adds to it. */
export function ReasonBadge({ reason }: { reason: string }) {
  return <Badge tone={isFailure(reason) ? "error" : "attention"}>{reason}</Badge>;
}
