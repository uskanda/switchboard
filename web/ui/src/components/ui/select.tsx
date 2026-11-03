import * as SelectPrimitive from "@radix-ui/react-select";
import { createContext, useContext, useEffect, useState, type ComponentProps } from "react";
import { cn } from "@/lib/utils";

// shadcn/ui's Select over Radix Select: a list box whose choices may hold more than text, such as a project's icon and
// its forge badge (DESIGN.md's 新規issue). The trigger looks like the Input; the list lies on the raised surface.

const OpenContext = createContext(false);

/** Select is Radix's root, which also tells SelectContent whether the list is open. */
export function Select({ open: openProp, defaultOpen, onOpenChange, ...props }: ComponentProps<typeof SelectPrimitive.Root>) {
  const [ownOpen, setOwnOpen] = useState(defaultOpen ?? false);
  const open = openProp ?? ownOpen;
  return (
    <OpenContext.Provider value={open}>
      <SelectPrimitive.Root
        open={open}
        onOpenChange={(o) => {
          setOwnOpen(o);
          onOpenChange?.(o);
        }}
        {...props}
      />
    </OpenContext.Provider>
  );
}

export const SelectValue = SelectPrimitive.Value;

export function SelectTrigger({ className, children, ...props }: ComponentProps<typeof SelectPrimitive.Trigger>) {
  return (
    <SelectPrimitive.Trigger
      className={cn(
        "flex h-11 w-full min-w-0 items-center justify-between gap-sm rounded-md border border-field-outline bg-surface-raised px-md text-left text-body-md focus-visible:outline-2 focus-visible:outline-primary disabled:opacity-60",
        className,
      )}
      {...props}
    >
      <span className="min-w-0 truncate">{children}</span>
      <SelectPrimitive.Icon asChild>
        <svg aria-hidden="true" viewBox="0 0 16 16" className="size-4 shrink-0 text-muted">
          <path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  );
}

/**
 * SelectContent is the open list. A modal <dialog> makes everything outside it inert, so inside one the list is put
 * into the dialog (container) rather than the body. While the list is open, the dialog's cancel is stopped: Escape
 * closes the list only, and leaves the dialog open. The stop outlasts the list by one task, as the browser may send
 * the cancel after the keydown that closed it.
 */
export function SelectContent({ className, children, container, ...props }: ComponentProps<typeof SelectPrimitive.Content> & { container?: HTMLElement | null }) {
  const open = useContext(OpenContext);
  useEffect(() => {
    if (!open || !(container instanceof HTMLDialogElement)) return;
    const stop = (e: Event) => e.preventDefault();
    container.addEventListener("cancel", stop);
    return () => {
      setTimeout(() => container.removeEventListener("cancel", stop), 0);
    };
  }, [open, container]);
  return (
    <SelectPrimitive.Portal container={container}>
      <SelectPrimitive.Content
        position="popper"
        sideOffset={4}
        className={cn(
          "z-50 max-h-[min(24rem,var(--radix-select-content-available-height))] w-[var(--radix-select-trigger-width)] overflow-hidden rounded-md border border-outline bg-surface-raised text-on-surface",
          className,
        )}
        {...props}
      >
        <SelectPrimitive.Viewport className="p-xs">{children}</SelectPrimitive.Viewport>
      </SelectPrimitive.Content>
    </SelectPrimitive.Portal>
  );
}

/**
 * SelectItem is one choice, as tall as a touch target; the chosen one is checked and the one under the keys is filled.
 * Radix names it through aria-labelledby, which would also read out what the content hides, such as the initial in a
 * project's icon; it is dropped, so the name comes from aria-label when one is given, else from the content.
 */
export function SelectItem({ className, children, ...props }: ComponentProps<typeof SelectPrimitive.Item>) {
  return (
    <SelectPrimitive.Item
      aria-labelledby={undefined}
      className={cn(
        "relative flex min-h-11 cursor-default items-center gap-sm rounded-sm py-xs pr-md pl-xl outline-hidden select-none data-[disabled]:opacity-60 data-[highlighted]:bg-surface data-[highlighted]:outline-2 data-[highlighted]:outline-solid data-[highlighted]:-outline-offset-2 data-[highlighted]:outline-primary",
        className,
      )}
      {...props}
    >
      <span className="absolute left-sm inline-flex size-4 items-center justify-center">
        <SelectPrimitive.ItemIndicator>
          <svg aria-hidden="true" viewBox="0 0 16 16" className="size-4 text-primary">
            <path d="M3 8.5l3 3 7-7" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </SelectPrimitive.ItemIndicator>
      </span>
      <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
    </SelectPrimitive.Item>
  );
}
