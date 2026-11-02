import { useCallback, useEffect, useState } from "react";

/** The list pane's width in pixels: DESIGN.md's 360px by default, between 280px and 640px. */
export const LIST_WIDTH_DEFAULT = 360;
export const LIST_WIDTH_MIN = 280;
export const LIST_WIDTH_MAX = 640;
/** The issue pane keeps at least this much, so that widening the list never squeezes the issue out. */
export const MAIN_WIDTH_MIN = 480;
/** How far one arrow key moves the border. */
export const LIST_WIDTH_STEP = 16;

const storageKey = "switchboard.listWidth";

/** clampListWidth bounds a width to the limits and to what a window of viewport pixels leaves the issue pane. */
export function clampListWidth(width: number, viewport: number): number {
  const max = Math.min(LIST_WIDTH_MAX, viewport - MAIN_WIDTH_MIN);
  return Math.round(Math.max(LIST_WIDTH_MIN, Math.min(width, max)));
}

/** readListWidth is the width stored as text, or the default when there is none or it is not a number. */
export function readListWidth(stored: string | null): number {
  const n = stored === null ? NaN : Number(stored);
  return Number.isFinite(n) && n > 0 ? n : LIST_WIDTH_DEFAULT;
}

function load(): number {
  try {
    return readListWidth(window.localStorage.getItem(storageKey));
  } catch {
    return LIST_WIDTH_DEFAULT;
  }
}

function save(width: number) {
  try {
    window.localStorage.setItem(storageKey, String(width));
  } catch {
    // A browser without storage keeps the width for this page only.
  }
}

/**
 * useListWidth is the list pane's width, remembered in localStorage across visits. setWidth bounds what it is given,
 * and the width shrinks again when the window narrows.
 */
export function useListWidth() {
  const [width, setRaw] = useState(() => clampListWidth(load(), window.innerWidth));

  const setWidth = useCallback((next: number) => {
    const w = clampListWidth(next, window.innerWidth);
    setRaw(w);
    save(w);
  }, []);

  useEffect(() => {
    const onResize = () => setRaw(clampListWidth(load(), window.innerWidth));
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  return [width, setWidth] as const;
}
