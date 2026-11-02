import { useLayoutEffect, useRef, type RefObject } from "react";

/** How long a row takes to move to its new place, or to fade in. */
export const flipDuration = 250;

const reduced = () => typeof window.matchMedia === "function" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/**
 * useFlip moves the rows under root (each an element with data-flip-key) from where they were drawn last to where
 * they are now, so a row that changes places, across sections too, is seen to move; a row that was not there fades
 * in, and one that went is gone at once. It measures after every render, relative to root, so a scroll between two
 * renders moves nothing. An element with no box (the copy that CSS hides, as the list draws both a table and cards)
 * is not measured. Until ready (the first listing), nothing is kept, so the first listing does not fade in. It does
 * nothing under prefers-reduced-motion, nor where elements have no animate.
 */
export function useFlip(root: RefObject<HTMLElement | null>, ready = true) {
  const last = useRef<Map<string, { x: number; y: number }> | null>(null);

  useLayoutEffect(() => {
    const el = root.current;
    if (!el || !ready) {
      last.current = null;
      return;
    }
    const origin = el.getBoundingClientRect();
    const rows: { key: string; node: HTMLElement; x: number; y: number }[] = [];
    for (const node of el.querySelectorAll<HTMLElement>("[data-flip-key]")) {
      const r = node.getBoundingClientRect();
      if (r.width === 0 && r.height === 0) continue;
      rows.push({ key: node.dataset.flipKey!, node, x: r.left - origin.left, y: r.top - origin.top });
    }
    const prev = last.current;
    last.current = new Map(rows.map((r) => [r.key, { x: r.x, y: r.y }]));
    if (!prev || reduced()) return;

    for (const { key, node, x, y } of rows) {
      if (typeof node.animate !== "function") return;
      const was = prev.get(key);
      if (!was) {
        node.animate([{ opacity: 0 }, { opacity: 1 }], { duration: flipDuration, easing: "ease-out" });
        continue;
      }
      const dx = was.x - x;
      const dy = was.y - y;
      if (dx === 0 && dy === 0) continue;
      node.animate([{ transform: `translate(${dx}px, ${dy}px)` }, { transform: "none" }], { duration: flipDuration, easing: "ease-out" });
    }
  });
}
