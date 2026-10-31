import type { ReactNode } from "react";
import { Link } from "react-router";

/** Shell is the page frame: the title bar and the centered column DESIGN.md sets at 720px. */
export function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="mx-auto min-h-dvh max-w-[720px] px-lg py-xl">
      <header className="mb-xl">
        <Link to="/" className="text-headline-md font-semibold">
          受信箱
        </Link>
      </header>
      <main className="flex flex-col gap-lg">{children}</main>
    </div>
  );
}
