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

/** Markdown draws issue text as Markdown, with raw HTML shown as text, HTML comments hidden and images as links. */
export function Markdown({ text }: { text: string }) {
  return (
    <div className="md break-words">
      <ReactMarkdown remarkPlugins={[remarkGfm, rawHTMLAsText]} components={components}>
        {text}
      </ReactMarkdown>
    </div>
  );
}
