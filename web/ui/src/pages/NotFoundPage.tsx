import { Link } from "react-router";
import { Shell } from "@/components/Shell";
import { defineMessages, useMessages } from "@/lib/i18n";
import { usePageTitle } from "@/lib/title";

/** messages is the text of the page for a URL that no route matches. */
const messages = defineMessages({
  ja: {
    title: "ページがありません",
    lead: "URLに当たる画面がありません。打ち間違いか、古いリンクかもしれません。",
    back: "一覧へ戻る",
  },
  en: {
    title: "Page not found",
    lead: "No page matches this URL. It may have a typo, or the link may be out of date.",
    back: "Back to inbox",
  },
});

/** NotFoundPage answers a URL that no route matches: it says so under the header and links back to the list. */
export function NotFoundPage() {
  const t = useMessages(messages);
  usePageTitle(t.title);
  return (
    <Shell>
      <h2 className="text-headline-md font-semibold">{t.title}</h2>
      <p className="text-muted">{t.lead}</p>
      <p>
        <Link to="/" className="inline-flex min-h-11 items-center underline focus-visible:outline-2 focus-visible:outline-primary">
          {t.back}
        </Link>
      </p>
    </Shell>
  );
}
