import { useEffect, useState } from "react";
import { ApiError, type Inbox } from "./api";

/** How the stream of the list stands: opening, open, or lost and being opened again by the browser. */
export type Connection = "connecting" | "live" | "lost";

export type LiveInbox = { inbox: Inbox | null; error: ApiError | null; connection: Connection };

/**
 * useLiveInbox follows the list through /api/events: switchboard-web sends the list when the stream opens and again
 * each time it changes. An "inbox" event carries the list, a "failure" event what kept switchboard-web from it.
 * The browser opens a lost stream again by itself.
 */
export function useLiveInbox(): LiveInbox {
  const [state, setState] = useState<LiveInbox>({ inbox: null, error: null, connection: "connecting" });

  useEffect(() => {
    const es = new EventSource("/api/events");
    es.onopen = () => setState((s) => ({ ...s, connection: "live" }));
    es.onerror = () => setState((s) => ({ ...s, connection: "lost" }));
    es.addEventListener("inbox", (e) => {
      const inbox = JSON.parse((e as MessageEvent<string>).data) as Inbox;
      setState({ inbox, error: null, connection: "live" });
    });
    es.addEventListener("failure", (e) => {
      const body = JSON.parse((e as MessageEvent<string>).data) as { code?: string; message?: string };
      const error = new ApiError(body.code ?? "unknown", body.message ?? "");
      setState((s) => ({ ...s, error, connection: "live" }));
    });
    return () => es.close();
  }, []);

  return state;
}
