import { useRef } from "react";
import { Link } from "react-router";
import { isDone, isQueued, isRunning, type Inbox } from "@/lib/api";
import { useHeightVar } from "@/lib/height";
import { useInbox, type Live } from "@/lib/inbox";

const liveText: Record<Live, string> = {
  connecting: "自動更新に接続しています",
  live: "自動更新中",
  reconnecting: "自動更新が切れています。再接続しています",
  off: "自動更新なし",
};

/**
 * statusText is the words for the state of the updates. While live, they also say how soon a change shows: at once
 * while the inbox service follows the relay, at the interval while it has lost the relay.
 */
function statusText(live: Live, inbox: Inbox | null): string {
  if (live !== "live") return liveText[live];
  if (inbox?.relay === "connected") return "自動更新中（即時）";
  if (inbox?.relay === "disconnected" && inbox.refresh_interval_seconds) return `自動更新中（${inbox.refresh_interval_seconds}秒ごと）`;
  return liveText.live;
}

/**
 * Header is the bar over every page: the name, the counts of waiting and running issues (and of queued and done ones
 * when there are any), the state of the automatic updates (with how soon a change shows, see statusText), and the
 * settings link. The name links to the list.
 */
export function Header() {
  const { inbox, live } = useInbox();
  const bar = useRef<HTMLElement>(null);
  useHeightVar(bar, "--header-h", document.documentElement);
  const running = inbox?.items.filter(isRunning).length ?? 0;
  const queued = inbox?.items.filter(isQueued).length ?? 0;
  const done = inbox?.items.filter(isDone).length ?? 0;
  const waiting = (inbox?.items.length ?? 0) - running - queued - done;
  const label = `待ち${waiting}件、実行中${running}件${queued > 0 ? `、順番待ち${queued}件` : ""}${done > 0 ? `、完了${done}件` : ""}`;
  return (
    <header ref={bar} className="sticky top-0 z-10 border-b border-outline bg-surface">
      <div className="flex min-h-14 flex-wrap items-center gap-x-lg gap-y-xs px-lg py-sm">
        <h1 className="text-headline-md font-semibold">
          <Link to="/" className="inline-flex min-h-11 items-center">
            受信箱
          </Link>
        </h1>
        {inbox && (
          <p className="text-label-md text-muted" aria-label={label}>
            待ち {waiting}・実行中 {running}
            {queued > 0 && `・順番待ち ${queued}`}
            {done > 0 && `・完了 ${done}`}
          </p>
        )}
        <p aria-live="polite" className="flex items-center gap-xs text-label-md text-muted">
          <span aria-hidden className={live === "live" ? "size-2 rounded-full bg-primary" : "size-2 rounded-full border border-muted"} />
          {statusText(live, inbox)}
        </p>
        <Link to="/settings" className="ml-auto inline-flex min-h-11 items-center text-label-md text-muted underline">
          設定
        </Link>
      </div>
    </header>
  );
}
