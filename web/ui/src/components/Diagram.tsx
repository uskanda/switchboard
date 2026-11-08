import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { useDarkScheme } from "@/lib/colorScheme";
import { diagramAllowed, withFocus } from "@/lib/diagram";
import { defineMessages, useMessages } from "@/lib/i18n";

const messages = defineMessages({
  ja: {
    drawing: "図を描いています",
    failed: "図を描けませんでした",
    source: "図の元の文",
    enlarge: "図を大きく見る",
    dialog: (n: number) => `Q${n}の図`,
    close: "閉じる",
  },
  en: {
    drawing: "Drawing the diagram",
    failed: "Couldn't draw the diagram",
    source: "The diagram's text",
    enlarge: "View the diagram larger",
    dialog: (n) => `Q${n} diagram`,
    close: "Close",
  },
});

// Each drawing gets an id of its own: Mermaid draws into a scratch element with that id.
let drawings = 0;

/** draw loads Mermaid, the first time a diagram is shown, and draws src as an SVG string. */
async function draw(src: string, dark: boolean): Promise<string> {
  const { default: mermaid } = await import("mermaid");
  mermaid.initialize({ startOnLoad: false, securityLevel: "strict", layout: "dagre", theme: dark ? "dark" : "neutral" });
  const color = getComputedStyle(document.documentElement).getPropertyValue("--color-primary").trim() || "currentColor";
  const { svg } = await mermaid.render(`diagram-${++drawings}`, withFocus(src, color));
  return svg;
}

/**
 * Svg puts the SVG Mermaid drew into the page. It is the one place where text that came by way of an issue becomes
 * markup (ADR-066): the diagram passed the checks before it was drawn, Mermaid's strict mode cleaned what it drew, and
 * the page's CSP still blocks scripts. The SVG is parsed apart with DOMParser, whose document runs no scripts, and
 * only its <svg> element is moved in; anything else is dropped.
 */
function Svg({ svg, className }: { svg: string; className: string }) {
  const at = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    const el = new DOMParser().parseFromString(svg, "text/html").body.querySelector("svg");
    if (el) at.current?.replaceChildren(document.adoptNode(el));
    else at.current?.replaceChildren();
  }, [svg]);
  return <span ref={at} className={className} />;
}

/** Fallback says the diagram could not be drawn, with its text folded. */
function Fallback({ source }: { source: string }) {
  const t = useMessages(messages);
  return (
    <div className="flex flex-col gap-xs">
      <p className="text-label-md text-muted">{t.failed}</p>
      <details className="group">
        <summary className="inline-flex min-h-11 cursor-pointer list-none items-center gap-xs rounded-sm text-label-md text-muted focus-visible:outline-2 focus-visible:outline-primary [&::-webkit-details-marker]:hidden">
          <span aria-hidden="true" className="transition-transform group-open:rotate-90">
            ▸
          </span>
          {t.source}
        </summary>
        <pre className="overflow-x-auto rounded-sm border border-outline p-sm font-mono text-mono-sm">
          <code>{source}</code>
        </pre>
      </details>
    </div>
  );
}

/**
 * Diagram draws the Mermaid diagram of question n, in the color scheme of the OS, drawn again when it changes. Only
 * a diagram that passes the same checks switchboard makes is drawn (ADR-066); one that does not, or that Mermaid
 * cannot read, is replaced by the words that it could not be drawn and its text, folded. The drawing fits the card's
 * width; pressing it opens it at its own size in a dialog that scrolls both ways. Mermaid is loaded only here, so a
 * page without a diagram never loads it.
 */
export function Diagram({ n, source }: { n: number; source: string }) {
  const t = useMessages(messages);
  const dark = useDarkScheme();
  const allowed = diagramAllowed(source);
  const [drawn, setDrawn] = useState<{ svg: string } | { failed: true } | null>(null);
  const [open, setOpen] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    if (!allowed) return;
    let stale = false;
    draw(source, dark).then(
      (svg) => !stale && setDrawn({ svg }),
      () => !stale && setDrawn({ failed: true }),
    );
    return () => void (stale = true);
  }, [source, dark, allowed]);

  useEffect(() => {
    const d = dialog.current;
    if (open && d && !d.open) d.showModal();
  }, [open]);

  if (!allowed || (drawn && "failed" in drawn)) return <Fallback source={source} />;
  if (!drawn) return <p className="text-label-md text-muted">{t.drawing}</p>;
  return (
    <>
      <button
        type="button"
        aria-label={t.enlarge}
        onClick={() => setOpen(true)}
        className="block w-full cursor-zoom-in rounded-sm focus-visible:outline-2 focus-visible:outline-primary"
      >
        <Svg svg={drawn.svg} className="block [&_svg]:mx-auto [&_svg]:block [&_svg]:h-auto [&_svg]:max-w-full" />
      </button>
      {open && (
        <Dialog ref={dialog} aria-label={t.dialog(n)} onClose={() => setOpen(false)} className="flex max-w-[calc(100vw-2*var(--spacing-lg))] flex-col gap-md">
          <div className="max-h-[70dvh] overflow-auto">
            <Svg svg={drawn.svg} className="block [&_svg]:max-w-none" />
          </div>
          <Button type="button" variant="secondary" className="self-end" onClick={() => dialog.current?.close()}>
            {t.close}
          </Button>
        </Dialog>
      )}
    </>
  );
}
