import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// A native <dialog> as DESIGN.md draws one: the surface with an outline, up to 720px wide, lg from the edges of a
// phone's screen. Open it with showModal() so that Escape closes it and the page behind takes no input.
export function Dialog({ className, ...props }: ComponentProps<"dialog">) {
  return (
    <dialog
      className={cn(
        "m-auto max-h-[calc(100dvh-2*var(--spacing-lg))] w-[calc(100%-2*var(--spacing-lg))] max-w-[720px] overflow-y-auto rounded-md border border-outline bg-surface p-lg text-on-surface backdrop:bg-black/40",
        className,
      )}
      {...props}
    />
  );
}
