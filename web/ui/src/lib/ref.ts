/** A forge the inbox reads issues from. */
export type Forge = "github" | "gitlab";

/** The forges as they spell their own names. */
export const forgeName: Record<Forge, string> = { github: "GitHub", gitlab: "GitLab" };

const prefixes: Record<string, Forge> = { "gh:": "github", "gl:": "gitlab" };

/**
 * splitRef takes the forge's prefix off a reference ("gh:o/r#12" → github and "o/r#12"), for the screen to show the
 * forge as a badge after the path. A string of another shape is given back as it is, without a forge. The reference
 * itself stays whole in links and requests; this is for display only.
 */
export function splitRef(ref: string): { forge?: Forge; path: string } {
  const forge = prefixes[ref.slice(0, 3)];
  const path = ref.slice(3);
  if (!forge || path === "") return { path: ref };
  return { forge, path };
}

/** refText is a reference as plain text, the forge's name after the path, for where a badge cannot go. */
export function refText(ref: string): string {
  const { forge, path } = splitRef(ref);
  return forge ? `${path} ${forgeName[forge]}` : path;
}
