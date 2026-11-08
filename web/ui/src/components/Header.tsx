import { useRef, type ReactNode } from "react";
import { Link } from "react-router";
import { isDone, isQueued, isRunning, isWaiting, type Inbox } from "@/lib/api";
import { useHeightVar } from "@/lib/height";
import { asOf, clockText, fullText } from "@/lib/freshness";
import { defineMessages, plural, useLang, useMessages } from "@/lib/i18n";
import { useAuth } from "@/lib/auth";
import { useInbox, type Live } from "@/lib/inbox";
import { useMinute } from "@/lib/now";

/** messages is the text of the header: the counts, the state of the automatic updates, the settings and the logout. */
const messages = defineMessages({
  ja: {
    /** live is the state of the automatic updates, as Live names it. */
    live: {
      connecting: "自動更新に接続しています",
      live: "自動更新中",
      reconnecting: "自動更新が切れています。再接続しています",
      off: "自動更新なし",
    } satisfies Record<Live, string>,
    instant: "自動更新中（即時）",
    every: (seconds: number) => `自動更新中（${seconds}秒ごと）`,
    asOf: (time: ReactNode) => <>{time}時点の一覧</>,
    /** counts is the name of the counts for a screen reader; queued and done are left out while there are none. */
    counts: (waiting: number, running: number, queued: number, done: number) =>
      `待ち${waiting}件、実行中${running}件${queued > 0 ? `、順番待ち${queued}件` : ""}${done > 0 ? `、完了${done}件` : ""}`,
    waiting: (n: number) => `待ち ${n}`,
    running: (n: number) => `実行中 ${n}`,
    queued: (n: number) => `順番待ち ${n}`,
    done: (n: number) => `完了 ${n}`,
    /** separator goes between the counts as they are shown. */
    separator: "・",
    settings: "設定",
    logout: "ログアウト",
  },
  en: {
    live: {
      connecting: "Connecting to live updates",
      live: "Live updates on",
      reconnecting: "Live updates lost. Reconnecting",
      off: "No live updates",
    },
    instant: "Live updates on (instant)",
    every: (seconds) => `Live updates on (every ${seconds}s)`,
    asOf: (time) => <>List as of {time}</>,
    counts: (waiting, running, queued, done) =>
      [
        `${waiting} ${plural(waiting, "issue", "issues")} waiting`,
        `${running} running`,
        ...(queued > 0 ? [`${queued} queued`] : []),
        ...(done > 0 ? [`${done} done`] : []),
      ].join(", "),
    waiting: (n) => `${n} waiting`,
    running: (n) => `${n} running`,
    queued: (n) => `${n} queued`,
    done: (n) => `${n} done`,
    separator: ", ",
    settings: "Settings",
    logout: "Log out",
  },
});

type Text = (typeof messages)["ja"];

/**
 * statusText is the words for the state of the updates. While live, they also say how soon a change shows: at once
 * while the inbox service follows the relay, at the interval while it has lost the relay.
 */
function statusText(t: Text, live: Live, inbox: Inbox | null): string {
  if (live !== "live") return t.live[live];
  if (inbox?.relay === "connected") return t.instant;
  if (inbox?.relay === "disconnected" && inbox.refresh_interval_seconds) return t.every(inbox.refresh_interval_seconds);
  return t.live.live;
}

/**
 * AsOf says the time the list was last known to be current, while the updates are not live and so the list may be
 * old: the later of the listing's fetched_at and the moment the updates stopped being live (lib/freshness.ts).
 */
function AsOf({ inbox, liveUntil }: { inbox: Inbox; liveUntil: string | null }) {
  const at = asOf(inbox.fetched_at, liveUntil);
  const lang = useLang();
  const t = useMessages(messages);
  // The clock text moves on each minute while the list stays as old as it is.
  useMinute();
  if (!at) return null;
  return (
    <span>
      {t.asOf(
        <time dateTime={at.toISOString()} title={fullText(lang, at)}>
          {clockText(lang, at, new Date())}
        </time>,
      )}
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
 * login on, the GitLab user logged in and the logout follow the settings link, which everyone gets: the settings page
 * holds the display section for everyone, and only its other sections are an admin's.
 */
export function Header() {
  const t = useMessages(messages);
  const { inbox, view, live, liveUntil } = useInbox();
  const { status, logout } = useAuth();
  const bar = useRef<HTMLElement>(null);
  useHeightVar(bar, "--header-h", document.documentElement);
  const running = view?.filter(isRunning).length ?? 0;
  const queued = view?.filter(isQueued).length ?? 0;
  const done = view?.filter(isDone).length ?? 0;
  const waiting = view?.filter(isWaiting).length ?? 0;
  const label = t.counts(waiting, running, queued, done);
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
              <span className={waiting > 0 ? "font-semibold text-attention" : undefined}>{t.waiting(waiting)}</span>
              {t.separator}
              {t.running(running)}
              {queued > 0 && `${t.separator}${t.queued(queued)}`}
              {done > 0 && `${t.separator}${t.done(done)}`}
            </span>
            <span className="sr-only">{label}</span>
          </p>
        )}
        <p aria-live="polite" className="flex items-center gap-xs text-label-md text-muted">
          <span aria-hidden className={live === "live" ? "size-2 rounded-full bg-primary" : "size-2 rounded-full border border-muted"} />
          {statusText(t, live, inbox)}
          {inbox && live !== "live" && <AsOf inbox={inbox} liveUntil={liveUntil} />}
        </p>
        </div>
        <div className="ml-auto flex items-center gap-lg">
          <Link to="/settings" className="inline-flex min-h-11 items-center text-label-md text-muted underline">
            {t.settings}
          </Link>
          {status.user && (
            <p className="flex items-center gap-sm text-label-md text-muted">
              <span>{status.user}</span>
              <button type="button" onClick={() => void logout()} className="inline-flex min-h-11 items-center underline">
                {t.logout}
              </button>
            </p>
          )}
        </div>
      </div>
    </header>
  );
}
