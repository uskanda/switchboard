import { useEffect } from "react";
import { isWaiting } from "@/lib/api";
import { useInbox } from "@/lib/inbox";

const app = "SwitchBoard Inbox";

/** The favicons in public/: the plain one, and the one with a dot that says an issue waits. */
const icons = { plain: "/favicon.svg", waiting: "/favicon-waiting.svg" };

/** setFavicon points the page's icon at href, adding the link where the page has none. */
function setFavicon(href: string) {
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!link) {
    link = document.createElement("link");
    link.rel = "icon";
    link.type = "image/svg+xml";
    document.head.appendChild(link);
  }
  if (link.getAttribute("href") !== href) link.setAttribute("href", href);
}

/**
 * usePageTitle names the tab after the screen (name, when given) and leads with the count of waiting issues while
 * there are any, so a tab in the background shows that something waits. The favicon says the same with a dot. An
 * issue replied to from this tab is counted where it is shown, under running (lib/pending.ts).
 */
export function usePageTitle(name?: string) {
  const { view } = useInbox();
  const waiting = view?.filter(isWaiting).length ?? 0;
  useEffect(() => {
    document.title = `${waiting > 0 ? `(${waiting}) ` : ""}${name ? `${name} - ` : ""}${app}`;
    setFavicon(waiting > 0 ? icons.waiting : icons.plain);
  }, [name, waiting]);
}
