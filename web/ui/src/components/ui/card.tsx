import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// shadcn/ui's Card, as DESIGN.md's card: the raised surface, an outline, no shadow.
export function Card({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("rounded-md border border-outline bg-surface-raised p-lg", className)} {...props} />;
}
