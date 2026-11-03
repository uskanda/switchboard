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

/** Shown is one item shown picks. */
export type Shown = ReturnType<typeof shown>[number];

/**
 * Round is an answered round of the thread, folded to one line: a wait of the agent's before its latest one, and the
 * people's comments after it up to the agent's next post.
 */
export type Round = { kind: "round"; question: Shown; replies: Shown[] };

/** ThreadItem is one item of the thread: a comment shown picks, or an answered round. */
export type ThreadItem = (Shown & { kind?: undefined }) | Round;

/**
 * threadOf lays out what the thread draws, oldest first: the items of shown, with every wait of the agent's before
 * its latest one gathered with the people's replies after it into a Round. A round ends at the agent's next post,
 * whatever its kind: the one-line claim of a run and the progress card stay outside it. The latest wait is never
 * folded, whatever the state of the issue.
 */
export function threadOf(comments: Comment[]): ThreadItem[] {
  const out: ThreadItem[] = [];
  let round: Round | null = null;
  for (const s of shown(comments)) {
    if (round && !s.c.agent) {
      round.replies.push(s);
      continue;
    }
    round = null;
    if (s.c.agent && s.c.kind === "waiting" && !s.latest) {
      round = { kind: "round", question: s, replies: [] };
      out.push(round);
    } else out.push(s);
  }
  return out;
}

/**
 * revealAndScroll scrolls the thread's card of the comment at index into view, below the pinned head. A card inside
 * a folded round opens the round first: a closed details hides it.
 */
export function revealAndScroll(root: ParentNode | null, index: number) {
  const el = root?.querySelector(`[data-index="${index}"]`);
  if (!el) return;
  for (let d = el.parentElement?.closest("details"); d; d = d.parentElement?.closest("details")) d.open = true;
  el.scrollIntoView({ block: "start" });
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
