import type { Comment, Detail } from "@/lib/api";

/** lastOf is the index of the agent's latest comment of kind, or -1. */
const lastOf = (comments: Comment[], kind: string) => comments.reduce((at, c, i) => (c.agent && c.kind === kind ? i : at), -1);

/**
 * shown picks the comments the thread draws: a claim of a run and a queued comment fold to one line, and of the
 * progress comments only the latest stays. The latest question of the agent is marked to stand out. i is the index of
 * the comment in comments.
 */
export function shown(comments: Comment[]) {
  const lastProgress = lastOf(comments, "progress");
  const lastQuestion = lastOf(comments, "waiting");
  return comments
    .map((c, i) => ({ c, i, folded: c.agent && (c.kind === "running" || c.kind === "queued") ? c.kind : undefined, latest: i === lastQuestion }))
    .filter(({ c, i }) => !(c.agent && c.kind === "progress" && i !== lastProgress));
}

/**
 * Card is one card the thread draws: index is its comment's index, key tells it from every other card. The key comes
 * from the author, the time and the text, so a rewritten progress comment is a new card.
 */
export type Card = { index: number; key: string };

/** cardsOf lists the cards the thread draws for comments, oldest first. */
export const cardsOf = (comments: Comment[]): Card[] =>
  shown(comments).map(({ c, i }) => ({ index: i, key: JSON.stringify([c.author, c.created_at, c.body]) }));

/** addedCards gives the comment indexes of the cards in next that were not in prev, oldest first. */
export function addedCards(prev: Card[], next: Card[]): number[] {
  const had = new Set(prev.map((c) => c.key));
  return next.filter((c) => !had.has(c.key)).map((c) => c.index);
}

/**
 * targetOf is the index of the comment an issue opens scrolled to: the agent's latest question for a waiting issue,
 * its latest done report (else the last card) for a done one. Other issues, and running or queued ones above all,
 * open at the top: null.
 */
export function targetOf(detail: Detail): number | null {
  if (detail.state === "running" || detail.state === "queued") return null;
  const comments = detail.comments ?? [];
  if (detail.waiting) {
    const i = lastOf(comments, "waiting");
    return i >= 0 ? i : null;
  }
  if (detail.done) {
    const i = lastOf(comments, "done");
    if (i >= 0) return i;
    const cards = shown(comments);
    return cards.length > 0 ? cards[cards.length - 1].i : null;
  }
  return null;
}
