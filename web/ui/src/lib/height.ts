import { useLayoutEffect, type RefObject } from "react";

/**
 * useHeightVar keeps the CSS variable name on target (the element itself when target is not given) at the height of
 * the element in ref, in px, as it wraps and changes. Pinned boxes stack under each other with it. A browser without
 * ResizeObserver (and jsdom) leaves the variable unset; the CSS falls back to 0px.
 */
export function useHeightVar(ref: RefObject<HTMLElement | null>, name: string, target?: HTMLElement) {
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const on = target ?? el;
    const set = () => on.style.setProperty(name, `${el.getBoundingClientRect().height}px`);
    set();
    const watch = new ResizeObserver(set);
    watch.observe(el);
    return () => {
      watch.disconnect();
      on.style.removeProperty(name);
    };
  }, [ref, name, target]);
}
