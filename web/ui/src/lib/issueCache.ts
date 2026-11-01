import type { Detail } from "@/lib/api";

/**
 * The last read of each issue opened in this tab, shown while the issue is read again. It lives in memory only: a
 * reload or a closed tab forgets it, and nothing of an issue is written to the browser's storage.
 */
const cache = new Map<string, Detail>();

/** MaxCachedIssues bounds the cache; the issue read longest ago goes first. */
export const MaxCachedIssues = 50;

/** cachedIssue is the last read of ref, or null when it has not been read in this tab. */
export const cachedIssue = (ref: string): Detail | null => cache.get(ref) ?? null;

/** rememberIssue keeps d as the last read of ref. */
export function rememberIssue(ref: string, d: Detail) {
  cache.delete(ref);
  cache.set(ref, d);
  for (const oldest of cache.keys()) {
    if (cache.size <= MaxCachedIssues) break;
    cache.delete(oldest);
  }
}

/** forgetIssues empties the cache. */
export const forgetIssues = () => cache.clear();
