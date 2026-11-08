import { useEffect, useState } from "react";

const DARK = "(prefers-color-scheme: dark)";

/** isDark reports whether the OS asks for a dark color scheme; a browser without matchMedia is taken as light. */
const isDark = () => typeof window.matchMedia === "function" && window.matchMedia(DARK).matches;

/** useDarkScheme follows the OS color scheme: true while it is dark, and it changes when the OS changes. */
export function useDarkScheme(): boolean {
  const [dark, setDark] = useState(isDark);
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const mq = window.matchMedia(DARK);
    const on = (e: { matches: boolean }) => setDark(e.matches);
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);
  return dark;
}
