/**
 * listScroll keeps how far the page was scrolled down the list. Below 1024px the page itself scrolls and the list and
 * an issue take turns on it, so coming back from an issue puts the list where it was left (issue #172). It lives in
 * memory only, like the drafts.
 */
let y = 0;

/** keepListScroll keeps v as how far the list was scrolled. */
export const keepListScroll = (v: number) => void (y = v);

/** listScroll is how far the list was scrolled when it was last shown, 0 at first. */
export const listScroll = () => y;

/** forgetListScroll starts over from the top, for the tests. */
export const forgetListScroll = () => void (y = 0);
