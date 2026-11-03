// The web inbox's address as the app keeps it: only the origin (scheme, host and port), since the WUI's pages all
// hang off its root and switchboard-web checks the Origin of every POST against its listen address.

export type Parsed = { ok: true; origin: string } | { ok: false; error: string };

const absolute = /^(https?):\/\/([^/?#\s@]+)(?:[/?#]\S*)?$/i;

/** parseWebURL reads what the user typed as the WUI's URL and keeps its origin. */
export function parseWebURL(input: string): Parsed {
  const m = absolute.exec(input.trim());
  if (!m || m[2].endsWith(":")) {
    return { ok: false, error: "http か https で始まる絶対URLを入力してください（例：http://192.168.0.10:8783）。" };
  }
  return { ok: true, origin: `${m[1].toLowerCase()}://${m[2].toLowerCase()}` };
}

/** isInside tells whether url is a page of the WUI at origin; anything else opens in the default browser. */
export function isInside(origin: string, url: string): boolean {
  if (!url.startsWith(origin)) return false;
  const rest = url.slice(origin.length);
  return rest === "" || rest.startsWith("/") || rest.startsWith("?") || rest.startsWith("#");
}

/** issuePath is the WUI's page of one issue; merge adds the query that opens its merge dialog. */
export function issuePath(ref: string, merge = false): string {
  return `/issue/${encodeURIComponent(ref)}${merge ? "?action=merge" : ""}`;
}
