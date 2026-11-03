import { useSearchParams } from "react-router";
import { Shell } from "@/components/Shell";
import { usePageTitle } from "@/lib/title";

/** reasonText says why the last login did not go through. */
function reasonText(error: string | null, user: string | null): string | null {
  switch (error) {
    case "not_allowed":
      return `GitLabのユーザー ${user ?? "?"} には、この受信箱を使う権限がありません。別のユーザーでログインするか、管理者に頼んでください。`;
    case "denied":
      return "GitLabで許可されませんでした。";
    case "failed":
      return "ログインできませんでした。もう一度試してください。";
  }
  return null;
}

/**
 * LoginPage is all the web inbox shows to someone not logged in while inbox.auth is set: why the last login failed,
 * if it did, and the one button that goes to GitLab. The button is a plain link: /auth/login answers with a redirect.
 */
export function LoginPage() {
  usePageTitle("ログイン");
  const [q] = useSearchParams();
  const reason = reasonText(q.get("error"), q.get("user"));
  return (
    <Shell header={false}>
      <h2 className="text-headline-md font-semibold">ログイン</h2>
      <p className="text-muted">この受信箱を使うには、GitLabでログインします。</p>
      {reason && (
        <p role="alert" className="rounded-sm bg-error-container p-sm text-on-error-container">
          {reason}
        </p>
      )}
      <p>
        <a
          href="/auth/login"
          className="inline-flex min-h-11 items-center rounded-md bg-primary px-lg text-label-md font-medium text-on-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        >
          GitLabでログイン
        </a>
      </p>
    </Shell>
  );
}
