import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// shadcn/ui's Textarea: the raised surface in an outline, with the primary outline on focus.
export function Textarea({ className, ...props }: ComponentProps<"textarea">) {
  return (
    <textarea
      className={cn(
        "min-h-32 w-full rounded-md border border-outline bg-surface-raised p-md text-body-md focus-visible:outline-2 focus-visible:outline-primary",
        className,
      )}
      {...props}
    />
  );
}
