import type { Item } from "@/lib/api";

/** A project's items in one section of the list. */
export type ProjectGroup<T extends Item = Item> = { project: string; items: T[] };

/**
 * projectRef is the project's reference of an issue's: the part before the last "#" ("gh:o/r#12" → "gh:o/r"). The
 * item's own project has no forge's prefix, so it cannot tell gh:o/r from gl:o/r; the reference can.
 */
export function projectRef(ref: string): string {
  const i = ref.lastIndexOf("#");
  return i < 0 ? ref : ref.slice(0, i);
}

/**
 * groupByProject puts items into one group per project, the groups in the order each project first comes and the
 * items in the order given. The inbox service gives each section oldest first, so the groups come in the order of
 * their oldest items without reading any time here.
 */
export function groupByProject<T extends Item>(items: T[]): ProjectGroup<T>[] {
  const groups = new Map<string, ProjectGroup<T>>();
  for (const it of items) {
    const project = projectRef(it.ref);
    let g = groups.get(project);
    if (!g) {
      g = { project, items: [] };
      groups.set(project, g);
    }
    g.items.push(it);
  }
  return [...groups.values()];
}
