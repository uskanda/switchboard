import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { IssueRef } from "@/components/IssueRef";
import { ApiError, getInbox, type Inbox } from "@/lib/api";
import { baseOf, loadMarks, overlay, pendingTimeout, saveMarks, settle, type Draft, type Mark, type PendingKind, type ViewItem } from "@/lib/pending";
import { toast } from "@/lib/toast";

/**
 * Live is the state of the automatic updates: connecting until the first listing of the stream, live while it
 * flows, reconnecting while the stream or the inbox service is down, and off where the browser has no EventSource.
 */
export type Live = "connecting" | "live" | "reconnecting" | "off";

export type InboxState = {
  /** The latest listing, null until the first one arrives. */
  inbox: Inbox | null;
  /** Why the first listing could not be read, if it could not. */
  error: string | null;
  live: Live;
  /**
   * When the updates last stopped being live (ISO 8601), null while they never were. Up to then the list was known to
   * be current, though fetched_at says only when it last changed (lib/freshness.ts).
   */
  liveUntil: string | null;
  /** The rows to show: the listing with what this tab asked of its issues laid over it (lib/pending.ts), null until the first listing. */
  view: ViewItem[] | null;
  /**
   * markPending marks an issue as being processed once a request for it worked, until the list shows the outcome.
   * A new issue ("create") takes the draft its row is drawn from; another kind is marked only while its issue is on
   * the list.
   */
  markPending: (ref: string, kind: PendingKind, draft?: Draft) => void;
};

type Feed = { inbox: Inbox | null; error: string | null; live: Live; liveUntil: string | null };

const InboxContext = createContext<InboxState>({ inbox: null, error: null, live: "off", liveUntil: null, view: null, markPending: () => {} });

/** leave moves the feed to another state of the updates, noting the time when it leaves live. */
const leave = (s: Feed, live: Live): Feed => ({ ...s, live, liveUntil: s.live === "live" ? new Date().toISOString() : s.liveUntil });

function failure(err: unknown): string {
  if (err instanceof ApiError && err.code === "unavailable") {
    return "受信箱のサービス（switchboard inbox serve）に届きません。サービスが動いているかを確かめてください。";
  }
  return `一覧を取れませんでした：${err instanceof Error ? err.message : String(err)}`;
}

/**
 * InboxProvider reads the listing once, then follows /api/events: each listing the stream sends replaces the one
 * shown. EventSource reconnects by itself after an error or an unavailable event; the state says so meanwhile.
 * While the page is hidden the stream is closed, so a tab left in the background keeps the inbox service from asking
 * the forges; when it shows again a new stream opens, and its first event is the listing of that moment. Each time
 * the updates stop being live the time is kept (liveUntil), so the header can say how old the list is meanwhile.
 * What this tab asked of the issues is laid over the listing (usePending).
 */
export function InboxProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<Feed>({ inbox: null, error: null, live: "off", liveUntil: null });

  useEffect(() => {
    let closed = false;
    getInbox().then(
      (inbox) => !closed && setState((s) => (s.inbox ? s : { ...s, inbox, error: null })),
      (err) => !closed && setState((s) => (s.inbox ? s : { ...s, error: failure(err) })),
    );
    if (typeof EventSource === "undefined") return () => void (closed = true);

    let es: EventSource | null = null;
    const open = () => {
      if (es) return;
      const source = new EventSource("/api/events");
      es = source;
      setState((s) => leave(s, "connecting"));
      source.addEventListener("inbox", (e) => {
        const inbox = JSON.parse((e as MessageEvent).data) as Inbox;
        setState((s) => ({ ...s, inbox, error: null, live: "live" }));
      });
      source.addEventListener("unavailable", () => setState((s) => leave(s, "reconnecting")));
      source.onerror = () => setState((s) => leave(s, "reconnecting"));
    };
    const close = () => {
      es?.close();
      es = null;
    };
    // A stream closed while hidden is no longer live: the list is as of this moment until the next stream's first
    // listing, which the page waits for once it shows again.
    const hide = () => {
      if (es) setState((s) => leave(s, "connecting"));
      close();
    };
    const follow = () => (document.visibilityState === "hidden" ? hide() : open());

    follow();
    document.addEventListener("visibilitychange", follow);
    return () => {
      closed = true;
      document.removeEventListener("visibilitychange", follow);
      close();
    };
  }, []);

  const pending = usePending(state.inbox);
  const value = useMemo(() => ({ ...state, ...pending }), [state, pending]);
  return <InboxContext.Provider value={value}>{children}</InboxContext.Provider>;
}

/**
 * usePending keeps the tab's marks against the listing. Each listing settles them; a mark that times out goes, with
 * a toast that says so. A timer set to the earliest deadline lets a mark time out while no listing arrives, and one
 * already past its deadline when the page loads goes at once.
 */
function usePending(inbox: Inbox | null) {
  const [marks, setMarks] = useState<Mark[]>(loadMarks);
  // tick moves when the earliest deadline is reached, so the marks are sorted again.
  const [tick, setTick] = useState(0);
  const latest = useRef(inbox);
  useEffect(() => void (latest.current = inbox), [inbox]);
  // told is the marks whose timeout was told already, so a second run of the effect does not tell it again.
  const told = useRef(new Set<string>());

  const sorted = useMemo(() => settle(marks, inbox?.items ?? null, Date.now()), [marks, inbox, tick]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (sorted.settled.length === 0 && sorted.expired.length === 0) return;
    for (const m of sorted.expired) {
      const key = `${m.ref}@${m.at}`;
      if (told.current.has(key)) continue;
      told.current.add(key);
      toast.warning(
        <>
          <IssueRef value={m.ref} /> の操作はまだ反映されていません
        </>,
      );
    }
    setMarks(sorted.keep);
  }, [sorted]);

  useEffect(() => saveMarks(marks), [marks]);

  useEffect(() => {
    if (sorted.keep.length === 0) return;
    const deadline = Math.min(...sorted.keep.map((m) => m.at)) + pendingTimeout;
    const timer = setTimeout(() => setTick((t) => t + 1), Math.max(0, deadline - Date.now()));
    return () => clearTimeout(timer);
  }, [sorted]);

  const markPending = useCallback((ref: string, kind: PendingKind, draft?: Draft) => {
    const it = latest.current?.items.find((i) => i.ref === ref);
    if (kind !== "create" && !it) return;
    const mark: Mark = { ref, kind, at: Date.now(), base: it ? baseOf(it) : null, ...(draft ? { draft } : {}) };
    setMarks((ms) => [...ms.filter((m) => m.ref !== ref), mark]);
  }, []);

  const view = useMemo(() => (inbox ? overlay(inbox.items, sorted.keep) : null), [inbox, sorted]);
  return useMemo(() => ({ view, markPending }), [view, markPending]);
}

export const useInbox = () => useContext(InboxContext);
