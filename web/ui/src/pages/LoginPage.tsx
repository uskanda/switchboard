import { useLocation, useSearchParams } from "react-router";
import { Shell } from "@/components/Shell";
import { defineMessages, setLanguageChoice, useLang, useMessages, type Lang } from "@/lib/i18n";
import { usePageTitle } from "@/lib/title";

/** messages is the text of the login page: why the last login failed, and the way to GitLab. */
const messages = defineMessages({
  ja: {
    title: "ログイン",
    lead: "この受信箱を使うには、GitLabでログインします。",
    notAllowed: (user: string) =>
      `GitLabのユーザー ${user} には、この受信箱を使う権限がありません。別のユーザーでログインするか、管理者に頼んでください。`,
    denied: "GitLabで許可されませんでした。",
    failed: "ログインできませんでした。もう一度試してください。",
    login: "GitLabでログイン",
  },
  en: {
    title: "Log in",
    lead: "Log in with GitLab to use this inbox.",
    notAllowed: (user) => `GitLab user ${user} has no access to this inbox. Log in as another user, or ask an admin for access.`,
    denied: "GitLab denied the login.",
    failed: "Could not log in. Try again.",
    login: "Log in with GitLab",
  },
});

/** The name of each language in itself, so the button that switches to it reads in the language it switches to. */
const languageNames: Record<Lang, string> = { ja: "日本語", en: "English" };

/** reasonText says why the last login did not go through. */
function reasonText(t: (typeof messages)["ja"], error: string | null, user: string | null): string | null {
  switch (error) {
    case "not_allowed":
      return t.notAllowed(user ?? "?");
    case "denied":
      return t.denied;
    case "failed":
      return t.failed;
  }
  return null;
}

/**
 * LoginPage is all the web inbox shows to someone not logged in while inbox.auth is set: why the last login failed,
 * if it did, and the one button that goes to GitLab. The button is a plain link: /auth/login answers with a redirect.
 * Opened on another page than the list (a link from a notification, say), the link asks the login to come back to it.
 * Under it, a button named in the other language shows the page in that language and keeps the choice, as the
 * settings page would, which no one can reach before logging in.
 */
export function LoginPage() {
  const t = useMessages(messages);
  const other: Lang = useLang() === "ja" ? "en" : "ja";
  usePageTitle(t.title);
  const [q] = useSearchParams();
  const reason = reasonText(t, q.get("error"), q.get("user"));
  const { pathname, search } = useLocation();
  const page = pathname + search;
  const href = pathname === "/" || pathname === "/login" ? "/auth/login" : `/auth/login?next=${encodeURIComponent(page)}`;
  return (
    <Shell header={false}>
      <h2 className="text-headline-md font-semibold">{t.title}</h2>
      <p className="text-muted">{t.lead}</p>
      {reason && (
        <p role="alert" className="rounded-sm bg-error-container p-sm text-on-error-container">
          {reason}
        </p>
      )}
      <p>
        <a
          href={href}
          className="inline-flex min-h-11 items-center rounded-md bg-primary px-lg text-label-md font-medium text-on-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        >
          {t.login}
        </a>
      </p>
      <p>
        <button
          type="button"
          lang={other}
          onClick={() => setLanguageChoice(other)}
          className="inline-flex min-h-11 items-center text-label-md text-muted underline focus-visible:outline-2 focus-visible:outline-primary"
        >
          {languageNames[other]}
        </button>
      </p>
    </Shell>
  );
}
