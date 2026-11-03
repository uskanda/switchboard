import type { Item } from "@/lib/api";

/**
 * The waits a reply was sent to in this tab, so the way to the next wait passes over an issue that stays on the list
 * until the agent takes the reply up. Like the drafts they live in memory only: a reload forgets them, and nothing is
 * written to the browser's storage.
 *
 * A wait is told by its issue, its start and its reason. watchKey would not do: the reply's comment changes the
 * issue's update time and comment count, while the wait it answered stays the same.
 */
const sent = new Set<string>();

const waitOf = (it: Item) => JSON.stringify([it.ref, it.waiting_since, it.reason]);

/** markSent records that a reply was sent to the wait it is in. */
export const markSent = (it: Item) => void sent.add(waitOf(it));

/** isSentWait is whether a reply was sent in this tab to the wait it is in; a wait begun since is not. */
export const isSentWait = (it: Item) => sent.has(waitOf(it));

/** forgetSent forgets every reply sent. */
export const forgetSent = () => sent.clear();
