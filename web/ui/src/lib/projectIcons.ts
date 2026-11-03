import { useEffect, useSyncExternalStore } from "react";
import { getProjectIcons } from "@/lib/api";
import { splitRef } from "@/lib/ref";

/**
 * The emoji icons of the configuration, by project reference, read once for the page and shared by every icon. A
 * failed read leaves the list empty: the icons are then the images, or the initials.
 */
let emoji: Record<string, string> = {};
let loading: Promise<void> | null = null;
const listeners = new Set<() => void>();

function load() {
  loading ??= getProjectIcons()
    .then((m) => {
      emoji = m ?? {};
    })
    .catch(() => {})
    .finally(() => listeners.forEach((l) => l()));
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

/** useProjectEmoji is the emoji icon of the project ref, or undefined when it has none (or the list is not read yet). */
export function useProjectEmoji(ref: string | null): string | undefined {
  useEffect(load, []);
  return useSyncExternalStore(subscribe, () => (ref ? emoji[ref] : undefined));
}

/** forgetProjectIcons drops the list read, so that the next page reads it again (the tests). */
export function forgetProjectIcons() {
  emoji = {};
  loading = null;
}

/**
 * projectOf is the project of a reference of an issue or a project ("gh:o/r#12" → "gh:o/r"), or null for a string
 * that is not a reference.
 */
export function projectOf(ref: string): string | null {
  if (!splitRef(ref).forge) return null;
  const i = ref.indexOf("#");
  return i < 0 ? ref : ref.slice(0, i);
}

/** initial is the first character of the project's name, the last part of its path, upper-cased: "gh:o/sandbox" → "S". */
export function initial(project: string): string {
  const name = splitRef(project).path.split("/").pop() ?? "";
  return (Array.from(name)[0] ?? "?").toUpperCase();
}
