import type { InstructionKind } from "@/lib/api";

/**
 * The instructions typed and not saved yet, by kind and target, so leaving the settings and coming back finds them
 * again. Like the drafts of the replies (lib/drafts.ts) they live in memory only: a reload forgets them, and nothing is
 * written to the browser's storage.
 */
const drafts = new Map<string, string>();

const keyOf = (kind: InstructionKind, target: string) => JSON.stringify([kind, target]);

/** instructionDraftOf is the text not saved of a field, or undefined when there is none. */
export const instructionDraftOf = (kind: InstructionKind, target: string): string | undefined => drafts.get(keyOf(kind, target));

/** keepInstructionDraft keeps text as the draft of a field; a text the same as the saved one drops it. */
export function keepInstructionDraft(kind: InstructionKind, target: string, text: string, saved: string) {
  if (text === saved) drafts.delete(keyOf(kind, target));
  else drafts.set(keyOf(kind, target), text);
}

/** forgetInstructionDrafts empties the drafts. */
export const forgetInstructionDrafts = () => drafts.clear();
