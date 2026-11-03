import type { ReactNode } from "react";
import { Header } from "@/components/Header";

/**
 * Shell is the frame of a one-column page, such as the settings: the header, then the 720px column DESIGN.md sets.
 * The login page has no header: it shows nothing of the inbox.
 */
export function Shell({ children, header = true }: { children: ReactNode; header?: boolean }) {
  return (
    <div className="min-h-dvh">
      {header && <Header />}
      <main className="mx-auto flex max-w-[720px] flex-col gap-lg px-lg py-xl">{children}</main>
    </div>
  );
}
