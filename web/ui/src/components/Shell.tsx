import type { ReactNode } from "react";
import { Link } from "react-router";

/**
 * Shell is the page frame: the title bar and the centered column DESIGN.md sets at 720px. On the list the title is
 * the page's heading; on the other pages it links back, and the page names its own heading. The settings link sits
 * at the right of the bar on every page.
 */
export function Shell({ children, home = false }: { children: ReactNode; home?: boolean }) {
  return (
    <div className="mx-auto min-h-dvh max-w-[720px] px-lg py-xl">
      <header className="mb-xl flex items-center justify-between gap-lg">
        {home ? (
          <h1 className="text-headline-md font-semibold">受信箱</h1>
        ) : (
          <Link to="/" className="inline-flex min-h-11 items-center text-headline-md font-semibold">
            ← 受信箱
          </Link>
        )}
        <Link to="/settings" className="inline-flex min-h-11 items-center text-label-md text-muted underline">
          設定
        </Link>
      </header>
      <main className="flex flex-col gap-lg">{children}</main>
    </div>
  );
}
