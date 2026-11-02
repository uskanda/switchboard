import { createElement, type ReactNode } from "react";
import { toast as sonner } from "sonner";

/**
 * toast.ts is the only door to sonner (DESIGN.md, Toast). Callers name a kind and a message; the rest of the web inbox
 * does not import sonner, so the library can change behind this file.
 *
 * The timers are kept here rather than in sonner: sonner pauses while the pointer is over the toasts but not while
 * the focus is in them, and WCAG 2.2.1 asks for both. The Toaster reports hover and focus with setHovered and
 * setFocused; sonner itself gets toasts that never close on their own.
 */

export type ToastKind = "info" | "success" | "warning" | "error";

export type ToastOptions = {
  /** A second line under the message. */
  description?: ReactNode;
  /** A link or button after the text, such as a link to the forge. */
  action?: ReactNode;
  /** Milliseconds before the toast goes away; 5000 when absent, Infinity to keep it until it is closed. */
  duration?: number;
};

export type ToastId = string | number;

/** The duration of a toast that names none. */
export const defaultDuration = 5000;

/** The most toasts shown at once; a sixth drops the oldest. */
export const maxToasts = 5;

type Entry = { remaining: number; started: number; timer?: ReturnType<typeof setTimeout> };

// Insertion order is age: the first key is the oldest toast.
const entries = new Map<ToastId, Entry>();
let hovered = false;
let focused = false;

const paused = () => hovered || focused;

function start(id: ToastId, e: Entry) {
  if (e.timer !== undefined || e.remaining === Infinity) return;
  e.started = Date.now();
  e.timer = setTimeout(() => dismiss(id), Math.max(0, e.remaining));
}

function stop(e: Entry) {
  if (e.timer === undefined) return;
  clearTimeout(e.timer);
  e.timer = undefined;
  e.remaining -= Date.now() - e.started;
}

function forget(id: ToastId) {
  const e = entries.get(id);
  if (!e) return;
  stop(e);
  entries.delete(id);
  // The toasts may vanish under the pointer or with the focus, and then no leave or blur comes.
  if (entries.size === 0) hovered = focused = false;
}

function update() {
  for (const [id, e] of entries) {
    if (paused()) stop(e);
    else start(id, e);
  }
}

/** setHovered says whether the pointer is over the toasts; the timers stand still while it is. */
export function setHovered(on: boolean) {
  hovered = on && entries.size > 0;
  update();
}

/** setFocused says whether the focus is in the toasts; the timers stand still while it is. */
export function setFocused(on: boolean) {
  focused = on && entries.size > 0;
  update();
}

function show(kind: ToastKind, message: ReactNode, opts: ToastOptions = {}): ToastId {
  const body = createElement(
    "div",
    { role: kind === "error" ? "alert" : "status", className: "flex min-w-0 flex-col gap-xs" },
    createElement("p", { className: "break-words" }, message),
    opts.description ? createElement("p", { className: "break-words text-label-md opacity-90" }, opts.description) : null,
    opts.action ?? null,
  );
  const id = sonner[kind](body, { duration: Infinity, onDismiss: (t) => forget(t.id), onAutoClose: (t) => forget(t.id) });
  const e: Entry = { remaining: opts.duration ?? defaultDuration, started: 0 };
  entries.set(id, e);
  if (!paused()) start(id, e);
  while (entries.size > maxToasts) {
    const oldest = entries.keys().next().value as ToastId;
    dismiss(oldest);
  }
  return id;
}

/** dismiss closes one toast, or every toast when no id is given. */
export function dismiss(id?: ToastId) {
  if (id === undefined) {
    for (const key of [...entries.keys()]) forget(key);
    sonner.dismiss();
    return;
  }
  forget(id);
  sonner.dismiss(id);
}

/** toast shows a message for a while at the bottom right: toast.success("保存しました"). */
export const toast = {
  info: (message: ReactNode, opts?: ToastOptions) => show("info", message, opts),
  success: (message: ReactNode, opts?: ToastOptions) => show("success", message, opts),
  warning: (message: ReactNode, opts?: ToastOptions) => show("warning", message, opts),
  error: (message: ReactNode, opts?: ToastOptions) => show("error", message, opts),
  dismiss,
};
