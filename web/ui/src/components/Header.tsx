import { Link } from "react-router";
import { isRunning } from "@/lib/api";
import { useInbox, type Live } from "@/lib/inbox";

const liveText: Record<Live, string> = {
  connecting: "自動更新に接続しています",
  live: "自動更新中",
  reconnecting: "自動更新が切れています。再接続しています",
  off: "自動更新なし",
};

/**
 * Header is the bar over every page: the name, the counts of waiting and running issues, the state of the automatic
 * updates, and the settings link. The name links to the list.
 */
export function Header() {
  const { inbox, live } = useInbox();
  const running = inbox?.items.filter(isRunning).length ?? 0;
  const waiting = (inbox?.items.length ?? 0) - running;
  return (
    <header className="sticky top-0 z-10 border-b border-outline bg-surface">
      <div className="flex min-h-14 flex-wrap items-center gap-x-lg gap-y-xs px-lg py-sm">
        <h1 className="text-headline-md font-semibold">
          <Link to="/" className="inline-flex min-h-11 items-center">
            受信箱
          </Link>
        </h1>
        {inbox && (
          <p className="text-label-md text-muted" aria-label={`待ち${waiting}件、実行中${running}件`}>
            待ち {waiting}・実行中 {running}
          </p>
        )}
        <p aria-live="polite" className="flex items-center gap-xs text-label-md text-muted">
          <span aria-hidden className={live === "live" ? "size-2 rounded-full bg-primary" : "size-2 rounded-full border border-muted"} />
          {liveText[live]}
        </p>
        <Link to="/settings" className="ml-auto inline-flex min-h-11 items-center text-label-md text-muted underline">
          設定
        </Link>
      </div>
    </header>
  );
}
