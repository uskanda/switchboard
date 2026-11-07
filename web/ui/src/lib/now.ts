import { useSyncExternalStore } from "react";

/**
 * The clock the times on the page ("5分前から待ち") are drawn by. One interval ticks each minute while anything uses
 * it, so a time moves on even when no new listing arrives.
 */
const listeners = new Set<() => void>();
let tick = 0;
let timer: ReturnType<typeof setInterval> | null = null;

function subscribe(listener: () => void) {
  listeners.add(listener);
  timer ??= setInterval(() => {
    tick++;
    listeners.forEach((l) => l());
  }, 60_000);
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  };
}

/** useMinute draws the component again each minute; what it returns only changes then. */
export const useMinute = () => useSyncExternalStore(subscribe, () => tick);
