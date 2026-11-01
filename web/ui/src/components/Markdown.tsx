import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

type Node = { type: string; value?: string; children?: Node[] };

const isComment = (n: Node) => n.type === "html" && /^<!--[\s\S]*-->$/.test(n.value?.trim() ?? "");

/**
 * rawHTMLAsText turns every piece of raw HTML in the Markdown into plain text: issue text is data, so a tag in it is
 * shown as the characters it is made of, never parsed. HTML comments, which issue templates are full of, are dropped.
 */
function rawHTMLAsText() {
  const walk = (n: Node) => {
    if (n.type === "html") n.type = "text";
    if (n.children) {
      n.children = n.children.filter((c) => !isComment(c));
      n.children.forEach(walk);
    }
  };
  return walk;
}

/**
 * lineBreaks draws a single line break inside a paragraph as a break, as GitHub does for issue text: options written
 * one to a line would otherwise run together. Code keeps its own line breaks. It runs after rawHTMLAsText, so that
 * HTML shown as text breaks its lines too.
 */
function lineBreaks() {
  const walk = (n: Node) => {
    if (!n.children) return;
    n.children = n.children.flatMap((c): Node[] => {
      if (c.type !== "text" || !c.value?.includes("\n")) return [c];
      return c.value.split("\n").flatMap((v, i): Node[] => [...(i ? [{ type: "break" }] : []), ...(v ? [{ type: "text", value: v }] : [])]);
    });
    n.children.forEach(walk);
  };
  return walk;
}

// Links from an issue open in a new tab and send no referrer. Images are not loaded: each becomes a link to it.
const components: Components = {
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="noreferrer" className="underline">
      {children}
    </a>
  ),
  img: ({ src, alt }) => (
    <a href={typeof src === "string" ? src : undefined} target="_blank" rel="noreferrer" className="underline">
      画像：{alt || String(src ?? "")}
    </a>
  ),
};

// Inline text, such as an option inside its label, keeps its phrasing without the paragraph around it.
const inlineComponents: Components = { ...components, p: ({ children }) => <>{children}</> };

/**
 * Markdown draws issue text as Markdown, with raw HTML shown as text, HTML comments hidden and images as links.
 * inline draws a single paragraph inside a span, for text that sits in a line such as a label.
 */
export function Markdown({ text, inline }: { text: string; inline?: boolean }) {
  const md = (
    <ReactMarkdown remarkPlugins={[remarkGfm, rawHTMLAsText, lineBreaks]} components={inline ? inlineComponents : components}>
      {text}
    </ReactMarkdown>
  );
  return inline ? <span className="md break-words">{md}</span> : <div className="md break-words">{md}</div>;
}
