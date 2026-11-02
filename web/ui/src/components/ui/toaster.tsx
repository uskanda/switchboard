import type { FocusEvent } from "react";
import { Toaster as Sonner } from "sonner";
// sonner inserts these rules as a <style> at run time, which the page's CSP (default-src 'self') blocks; the build
// carries them in the stylesheet instead, or the toasts are left unpositioned below the page.
import "sonner/dist/styles.css";
import { setFocused, setHovered } from "@/lib/toast";

// The marks before the text, drawn in the color of the text so they follow each kind and the dark theme.
const mark = (path: string) => (
  <svg viewBox="0 0 20 20" width="20" height="20" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="10" cy="10" r="8" />
    <path d={path} />
  </svg>
);

const icons = {
  info: mark("M10 9v5M10 6.2v.1"),
  success: mark("M6.5 10.2l2.4 2.4 4.6-4.9"),
  warning: mark("M10 5.8v5M10 13.8v.1"),
  error: mark("M7.4 7.4l5.2 5.2M12.6 7.4l-5.2 5.2"),
  close: (
    <svg viewBox="0 0 20 20" width="18" height="18" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
      <path d="M5.5 5.5l9 9M14.5 5.5l-9 9" />
    </svg>
  ),
};

/**
 * Toaster is where the toasts of lib/toast.ts appear: at the bottom right, the newest at the bottom, all five spread
 * out; below 1024px across the bottom with lg on each side (DESIGN.md, Toast). sonner draws them unstyled, and the
 * classes below are DESIGN.md's toast-info, toast-success, toast-warning and toast-error. Only this file and
 * lib/toast.ts import sonner.
 */
export function Toaster() {
  const blur = (e: FocusEvent<HTMLDivElement>) => {
    if (!e.currentTarget.contains(e.relatedTarget)) setFocused(false);
  };
  return (
    <div className="contents" onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)} onFocus={() => setFocused(true)} onBlur={blur}>
      <Sonner
        position="bottom-right"
        expand
        visibleToasts={5}
        closeButton
        offset={16}
        mobileOffset={16}
        gap={8}
        containerAriaLabel="通知"
        icons={icons}
        className="max-lg:inset-x-lg! max-lg:w-auto!"
        style={{ "--width": "360px" } as React.CSSProperties}
        toastOptions={{
          unstyled: true,
          closeButtonAriaLabel: "閉じる",
          classNames: {
            toast:
              "flex w-full! items-start gap-sm rounded-md border py-sm ps-md pe-sm text-label-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary",
            info: "toast-info border-outline bg-surface-raised text-on-surface",
            success: "toast-success border-transparent bg-success-container text-on-success-container [&_[data-icon]]:text-success",
            warning: "toast-warning border-transparent bg-attention-container text-attention",
            error: "toast-error border-transparent bg-error-container text-on-error-container",
            icon: "mt-[10px] flex shrink-0",
            content: "min-w-0 flex-1 py-[10px]",
            closeButton:
              "order-last inline-flex size-11 shrink-0 cursor-pointer items-center justify-center rounded-md focus-visible:outline-2 focus-visible:outline-primary",
          },
        }}
      />
    </div>
  );
}
