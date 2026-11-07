import { useRef } from "react";
import { Link } from "react-router";
import { isDone, isQueued, isRunning, isWaiting, type Inbox } from "@/lib/api";
import { useHeightVar } from "@/lib/height";
import { asOf, clockText, fullText } from "@/lib/freshness";
import { useAuth } from "@/lib/auth";
import { useInbox, type Live } from "@/lib/inbox";
import { useMinute } from "@/lib/now";

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
 * AsOf says the time the list was last known to be current, while the updates are not live and so the list may be
 * old: the later of the listing's fetched_at and the moment the updates stopped being live (lib/freshness.ts).
 */
function AsOf({ inbox, liveUntil }: { inbox: Inbox; liveUntil: string | null }) {
  const t = asOf(inbox.fetched_at, liveUntil);
  // The clock text moves on each minute while the list stays as old as it is.
  useMinute();
  if (!t) return null;
  return (
    <span>
      <time dateTime={t.toISOString()} title={fullText(t)}>
        {clockText(t, new Date())}
      </time>
      時点の一覧
    </span>
  );
}

/**
 * Header is the bar over every page: the name, the counts of waiting and running issues (and of queued and done ones
 * when there are any), the state of the automatic updates (with how soon a change shows, see statusText), and the
 * settings link. While the updates are not live, the state is followed by the time of the list (AsOf). The name links
 * to the list. While an issue waits, its count is bold in the attention color. On a narrow screen the settings link
 * stays on the first line, beside the name, and the counts and the state wrap below them. The counts are of the rows
 * as the list shows them, with what this tab asked of the issues laid over the listing (lib/pending.ts). With the
 * login on, the GitLab user logged in and the logout follow the settings link, which only an admin gets.
 */
export function Header() {
  const { inbox, view, live, liveUntil } = useInbox();
  const { status, logout } = useAuth();
  const bar = useRef<HTMLElement>(null);
  useHeightVar(bar, "--header-h", document.documentElement);
  const running = view?.filter(isRunning).length ?? 0;
  const queued = view?.filter(isQueued).length ?? 0;
  const done = view?.filter(isDone).length ?? 0;
  const waiting = view?.filter(isWaiting).length ?? 0;
  const label = `待ち${waiting}件、実行中${running}件${queued > 0 ? `、順番待ち${queued}件` : ""}${done > 0 ? `、完了${done}件` : ""}`;
  return (
    <header ref={bar} className="sticky top-0 z-10 border-b border-outline bg-surface">
      <div className="flex min-h-14 flex-wrap items-center gap-x-lg gap-y-xs px-lg py-sm">
        <h1 className="text-headline-md font-semibold">
          <Link to="/" className="inline-flex min-h-11 items-center">
            <span>
              SwitchBoard<span className="hidden sm:inline"> Inbox</span>
            </span>
          </Link>
        </h1>
        <div className="flex flex-wrap items-center gap-x-lg gap-y-xs max-sm:order-1 max-sm:w-full">
        {inbox && (
          <p className="text-label-md text-muted">
            <span aria-hidden>
              <span className={waiting > 0 ? "font-semibold text-attention" : undefined}>待ち {waiting}</span>・実行中 {running}
              {queued > 0 && `・順番待ち ${queued}`}
              {done > 0 && `・完了 ${done}`}
            </span>
            <span className="sr-only">{label}</span>
          </p>
        )}
        <p aria-live="polite" className="flex items-center gap-xs text-label-md text-muted">
          <span aria-hidden className={live === "live" ? "size-2 rounded-full bg-primary" : "size-2 rounded-full border border-muted"} />
          {statusText(live, inbox)}
          {inbox && live !== "live" && <AsOf inbox={inbox} liveUntil={liveUntil} />}
        </p>
        </div>
        <div className="ml-auto flex items-center gap-lg">
          {(!status.enabled || status.admin) && (
            <Link to="/settings" className="inline-flex min-h-11 items-center text-label-md text-muted underline">
              設定
            </Link>
          )}
          {status.user && (
            <p className="flex items-center gap-sm text-label-md text-muted">
              <span>{status.user}</span>
              <button type="button" onClick={() => void logout()} className="inline-flex min-h-11 items-center underline">
                ログアウト
              </button>
            </p>
          )}
        </div>
      </div>
    </header>
  );
}
