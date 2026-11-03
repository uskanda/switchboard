import { Link } from "react-router";
import { Shell } from "@/components/Shell";
import { usePageTitle } from "@/lib/title";

/** NotFoundPage answers a URL that no route matches: it says so under the header and links back to the list. */
export function NotFoundPage() {
  usePageTitle("ページがありません");
  return (
    <Shell>
      <h2 className="text-headline-md font-semibold">ページがありません</h2>
      <p className="text-muted">URLに当たる画面がありません。打ち間違いか、古いリンクかもしれません。</p>
      <p>
        <Link to="/" className="inline-flex min-h-11 items-center underline focus-visible:outline-2 focus-visible:outline-primary">
          一覧へ戻る
        </Link>
      </p>
    </Shell>
  );
}
