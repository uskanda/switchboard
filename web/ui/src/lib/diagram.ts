// The checks on a question's Mermaid diagram before it is drawn. switchboard checks the same when it writes the
// question (internal/agent/planrun.go, diagramOf); the web inbox checks again, as the page allows inline styles
// (ADR-066) and a diagram may say what to draw, never how.

const KINDS = ["flowchart", "stateDiagram-v2", "sequenceDiagram"];
// Words at the start of a line that would restyle the page or run something.
const STYLES = ["classDef", "style", "linkStyle", "click"];

/** diagramAllowed reports whether src is one of the three kinds and holds no line that restyles and no directive. */
export function diagramAllowed(src: string): boolean {
  const text = src.trim();
  if (text === "" || text.includes("%%{")) return false;
  if (!KINDS.includes(text.split(/\s+/)[0])) return false;
  return !text.split("\n").some((l) => STYLES.includes(l.trim().split(/\s+/)[0]));
}

/**
 * withFocus defines the class :::focus marks with, right after the first line: a thicker outline in color, the
 * page's primary color, so the node a question is about stands out without a color of its own. A sequence diagram
 * has no classes and is left as it is.
 */
export function withFocus(src: string, color: string): string {
  const [first, ...rest] = src.trim().split("\n");
  if (first.trim().startsWith("sequenceDiagram")) return src.trim();
  return [first, `  classDef focus stroke-width:3px,stroke:${color}`, ...rest].join("\n");
}
