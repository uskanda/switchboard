import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { ApiError, getInbox, type Inbox } from "@/lib/api";

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
};

const InboxContext = createContext<InboxState>({ inbox: null, error: null, live: "off" });

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
 * the forges; when it shows again a new stream opens, and its first event is the listing of that moment.
 */
export function InboxProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<InboxState>({ inbox: null, error: null, live: "off" });

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
      setState((s) => ({ ...s, live: "connecting" }));
      source.addEventListener("inbox", (e) => {
        const inbox = JSON.parse((e as MessageEvent).data) as Inbox;
        setState({ inbox, error: null, live: "live" });
      });
      source.addEventListener("unavailable", () => setState((s) => ({ ...s, live: "reconnecting" })));
      source.onerror = () => setState((s) => ({ ...s, live: "reconnecting" }));
    };
    const close = () => {
      es?.close();
      es = null;
    };
    const follow = () => (document.visibilityState === "hidden" ? close() : open());

    follow();
    document.addEventListener("visibilitychange", follow);
    return () => {
      closed = true;
      document.removeEventListener("visibilitychange", follow);
      close();
    };
  }, []);

  return <InboxContext.Provider value={state}>{children}</InboxContext.Provider>;
}

export const useInbox = () => useContext(InboxContext);
