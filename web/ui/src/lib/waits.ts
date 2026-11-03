import type { Item } from "@/lib/api";
import { isSentWait } from "@/lib/sent";

/**
 * nextWait is the wait to go to once a reply to current is sent: the first one below current in waiting (the list's
 * waits, in its order), going round from the end to the top. It passes over current and the waits a reply was sent
 * to in this tab; it is null when no other wait is left.
 */
export function nextWait(waiting: Item[], current: string | undefined): Item | null {
  const at = waiting.findIndex((it) => it.ref === current);
  for (let i = 1; i <= waiting.length; i++) {
    const it = waiting[(at + i) % waiting.length];
    if (it.ref !== current && !isSentWait(it)) return it;
  }
  return null;
}

/**
 * stepWait is the wait one below (dir 1) or one above (dir -1) current in waiting, stopping at the ends. With no wait
 * open, it is the first wait going down and the last going up. It is null when there is nowhere to go.
 */
export function stepWait(waiting: Item[], current: string | undefined, dir: 1 | -1): Item | null {
  if (waiting.length === 0) return null;
  const at = waiting.findIndex((it) => it.ref === current);
  if (at < 0) return dir === 1 ? waiting[0] : waiting[waiting.length - 1];
  return waiting[at + dir] ?? null;
}

/**
 * ignoreKey is whether a key of the page's own (j, k, r) is left alone: pressed with a modifier, typed in a field, or
 * pressed while a dialog is open.
 */
export function ignoreKey(e: KeyboardEvent): boolean {
  if (e.ctrlKey || e.altKey || e.metaKey) return true;
  const t = e.target;
  if (t instanceof HTMLElement && (["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName) || t.isContentEditable)) return true;
  return document.querySelector("dialog[open]") !== null;
}
