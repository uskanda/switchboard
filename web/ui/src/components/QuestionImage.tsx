import { createContext, useContext, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { defineMessages, useMessages } from "@/lib/i18n";
import type { Image } from "@/lib/planQuestions";

const messages = defineMessages({
  ja: {
    link: (name: string) => `画像：${name}`,
    enlarge: (name: string) => `${name}を大きく見る`,
    dialog: (name: string) => `${name}の画像`,
    close: "閉じる",
  },
  en: {
    link: (name) => `Image: ${name}`,
    enlarge: (name) => `View ${name} larger`,
    dialog: (name) => `${name} image`,
    close: "Close",
  },
});

/**
 * IssueImages names the issue whose question comment is drawn, and the web address of its project, so that the
 * pictures of its questions are read through this server (/api/issue-image) and fall back to links to the forge.
 */
export type IssueImages = { issue: string; base: string };

export const IssueImagesContext = createContext<IssueImages | null>(null);

/**
 * projectBase is the web address of the project an issue's address is in: GitLab's /-/issues/ and /-/work_items/, and
 * GitHub's /issues/, cut off.
 */
export const projectBase = (issueUrl: string) => issueUrl.replace(/\/-\/(?:issues|work_items)\/\d+.*$/, "").replace(/\/issues\/\d+.*$/, "");

/** href is where the link to an image goes: the forge's address, for one written relative to the project. */
const href = (src: string, base: string) => (src.startsWith("/") && !src.startsWith("//") && base ? base + src : src);

/**
 * QuestionImage draws a picture of a plan question (ADR-069), read through this server from the issue's GitLab
 * project with the inbox token, so that the page's CSP stays img-src 'self'. It fits the card's width, and a tall one
 * a height of 60% of the screen at most; pressing it
 * opens it at its own size in a dialog that scrolls both ways. A picture that cannot be read, or one drawn outside an
 * issue, is a link to it, as every other image of issue text is.
 */
export function QuestionImage({ image, name }: { image: Image; name: string }) {
  const t = useMessages(messages);
  const at = useContext(IssueImagesContext);
  const [failed, setFailed] = useState(false);
  const [open, setOpen] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = dialog.current;
    if (open && d && !d.open) d.showModal();
  }, [open]);
  const alt = image.alt || name;
  if (!at || failed) {
    return (
      <a href={href(image.src, at?.base ?? "")} target="_blank" rel="noreferrer" className="self-start underline">
        {t.link(alt)}
      </a>
    );
  }
  const src = `/api/issue-image?${new URLSearchParams({ issue: at.issue, src: image.src })}`;
  return (
    <>
      <button
        type="button"
        aria-label={t.enlarge(alt)}
        onClick={() => setOpen(true)}
        className="block w-full cursor-zoom-in rounded-sm border border-outline focus-visible:outline-2 focus-visible:outline-primary"
      >
        <img src={src} alt={alt} loading="lazy" onError={() => setFailed(true)} className="mx-auto block h-auto max-h-[min(60dvh,32rem)] w-auto max-w-full rounded-sm" />
      </button>
      {open && (
        <Dialog ref={dialog} aria-label={t.dialog(alt)} onClose={() => setOpen(false)} className="flex max-w-[calc(100vw-2*var(--spacing-lg))] flex-col gap-md">
          <div className="max-h-[70dvh] overflow-auto">
            <img src={src} alt={alt} className="block max-w-none" />
          </div>
          <Button type="button" variant="secondary" className="self-end" onClick={() => dialog.current?.close()}>
            {t.close}
          </Button>
        </Dialog>
      )}
    </>
  );
}
