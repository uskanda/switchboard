import type { Answer } from "@/lib/planQuestions";

/** Draft is what was typed and chosen for an issue's reply and not sent yet. */
export type Draft = { text: string; choices: { key: string; answers: Answer[] } | null };

/**
 * The drafts of the replies, by issue, so moving to another issue and back keeps what was typed. Like the issue cache
 * they live in memory only: a reload forgets them, and nothing is written to the browser's storage.
 */
const drafts = new Map<string, Draft>();

const empty: Draft = { text: "", choices: null };

/** draftOf is the draft of ref, empty when there is none. */
export const draftOf = (ref: string): Draft => drafts.get(ref) ?? empty;

/** keepDraft keeps d as the draft of ref; an empty one is dropped. */
export function keepDraft(ref: string, d: Draft) {
  if (d.text === "" && d.choices === null) drafts.delete(ref);
  else drafts.set(ref, d);
}

/** forgetDrafts empties the drafts. */
export const forgetDrafts = () => drafts.clear();
