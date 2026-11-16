import type { ComponentProps } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";

// A native <dialog> as DESIGN.md draws one: the surface with an outline, up to 720px wide, lg from the edges of a
// phone's screen. Open it with showModal() so that Escape closes it and the page behind takes no input.
//
// It is drawn into document.body, outside the part of the page that moves. A browser takes a modal dialog out of the
// top layer when an ancestor is moved in the DOM (React moves a row when a listing reorders the rows) and leaves it
// open but not modal: hidden in its row, deaf to Escape, and showModal on it throws, so the button that opens it does
// nothing. Events still bubble along React's tree, as before.
export function Dialog({ className, ...props }: ComponentProps<"dialog">) {
  return createPortal(
    <dialog
      className={cn(
        "m-auto max-h-[calc(100dvh-2*var(--spacing-lg))] w-[calc(100%-2*var(--spacing-lg))] max-w-[720px] overflow-y-auto rounded-md border border-outline bg-surface p-lg text-on-surface backdrop:bg-black/40",
        className,
      )}
      {...props}
    />,
    document.body,
  );
}
