import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// shadcn/ui's Input: one line in the raised surface and outline of the textarea, as tall as a touch target.
export function Input({ className, ...props }: ComponentProps<"input">) {
  return (
    <input
      className={cn(
        "h-11 w-full min-w-0 rounded-md border border-outline bg-surface-raised px-md text-body-md focus-visible:outline-2 focus-visible:outline-primary",
        className,
      )}
      {...props}
    />
  );
}
