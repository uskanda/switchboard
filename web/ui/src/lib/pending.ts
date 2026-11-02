import type { Item } from "@/lib/api";

/**
 * pending.ts keeps what was asked of an issue from this tab until the list shows its outcome: a merge, a reply (an
 * approval, an answer, a run again, a comment on a done issue) or a new issue. Until then the row shows that it is
 * being processed, in the section it is expected to move to: a reply and a new issue under running, a merge still
 * under done, since a closed issue leaves the list. The marks live in the tab's sessionStorage, so a reload keeps
 * them; other tabs and machines do not see them.
 */

export type PendingKind = "merge" | "reply" | "create";

/** Draft is what a new issue's row is drawn from until the list has the issue: the answer to the creation and its title. */
export type Draft = { title: string; url: string; project: string; number: number };

/**
 * Mark is one request: the issue, what was asked, when (milliseconds since the epoch), and base, the issue's state
 * on the list at that time (baseOf), or null for a new issue not on the list.
 */
export type Mark = { ref: string; kind: PendingKind; at: number; base: string | null; draft?: Draft };

/** ViewItem is a row as shown: an item, marked with what is pending on it. */
export type ViewItem = Item & { pending?: PendingKind };

/** A mark the list has not settled in this long is dropped, and the row goes back to its state on the list. */
export const pendingTimeout = 10 * 60_000;

export const storageKey = "switchboard.pending";

function since(it: Item): string | null {
  switch (it.state) {
    case "done":
      return it.done_since ?? null;
    case "running":
      return it.running_since ?? null;
    case "queued":
      return it.queued_since ?? null;
    default:
      return it.waiting_since;
  }
}

/**
 * baseOf is an issue's state on the list and when that state began. The start counts, so a plan run that ends
 * before the next listing, waiting again, counts as a change.
 */
export const baseOf = (it: Item) => JSON.stringify([it.state ?? "waiting", since(it)]);

/**
 * settle sorts the marks by the listing (null before the first one): settled once the issue's base moves, the
 * issue leaves the list, or a new issue shows on it; expired once pendingTimeout has gone by; kept otherwise.
 */
export function settle(marks: Mark[], items: Item[] | null, now: number) {
  const keep: Mark[] = [];
  const settled: Mark[] = [];
  const expired: Mark[] = [];
  const byRef = new Map(items?.map((it) => [it.ref, it]));
  for (const m of marks) {
    const it = byRef.get(m.ref);
    const done = items !== null && (m.kind === "create" ? !!it : !it || baseOf(it) !== m.base);
    if (done) settled.push(m);
    else if (now - m.at >= pendingTimeout) expired.push(m);
    else keep.push(m);
  }
  return { keep, settled, expired };
}

/**
 * overlay is the rows to show: a replied issue as running, since the mark; a merged one done; each marked pending.
 * A new issue not on the list yet gets a running row of its own, after the others, with no machine.
 */
export function overlay(items: Item[], marks: Mark[]): ViewItem[] {
  if (marks.length === 0) return items;
  const byRef = new Map(marks.map((m) => [m.ref, m]));
  const rows: ViewItem[] = items.map((it) => {
    const m = byRef.get(it.ref);
    if (!m || m.kind === "create") return it;
    if (m.kind === "merge") return { ...it, pending: "merge" };
    return { ...it, state: "running", running_since: new Date(m.at).toISOString(), pending: "reply" };
  });
  const listed = new Set(items.map((it) => it.ref));
  for (const m of marks) {
    if (m.kind !== "create" || !m.draft || listed.has(m.ref)) continue;
    rows.push({
      ref: m.ref,
      project: m.draft.project,
      number: m.draft.number,
      title: m.draft.title,
      url: m.draft.url,
      host: "",
      waiting_since: null,
      reason: "",
      question: "",
      state: "running",
      running_since: new Date(m.at).toISOString(),
      pending: "create",
    });
  }
  return rows;
}

const kinds: PendingKind[] = ["merge", "reply", "create"];

function isDraft(v: unknown): v is Draft {
  const d = v as Draft;
  return typeof d === "object" && d !== null && typeof d.title === "string" && typeof d.url === "string" && typeof d.project === "string" && typeof d.number === "number";
}

function isMark(v: unknown): v is Mark {
  const m = v as Mark;
  return (
    typeof m === "object" &&
    m !== null &&
    typeof m.ref === "string" &&
    kinds.includes(m.kind) &&
    typeof m.at === "number" &&
    (typeof m.base === "string" || m.base === null) &&
    (m.draft === undefined || isDraft(m.draft))
  );
}

/** loadMarks reads the tab's marks; a stored value of another shape is dropped. */
export function loadMarks(): Mark[] {
  try {
    const v: unknown = JSON.parse(sessionStorage.getItem(storageKey) ?? "[]");
    return Array.isArray(v) ? v.filter(isMark) : [];
  } catch {
    return [];
  }
}

/** saveMarks keeps the tab's marks, and removes the key once none is left. */
export function saveMarks(marks: Mark[]) {
  try {
    if (marks.length === 0) sessionStorage.removeItem(storageKey);
    else sessionStorage.setItem(storageKey, JSON.stringify(marks));
  } catch {
    // A storage that is full or turned off keeps the marks in memory only.
  }
}
