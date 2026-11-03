import { issuePath } from "./url";

// What a push notification carries, as the inbox service writes it into the FCM data's body (internal/push): the
// issue, its state and reason, and the start of the post it reports on. No question or plan is in it.
export type Payload = {
  ref: string;
  state: string;
  reason?: string;
  since: string;
  change_number?: number;
  change_sha?: string;
};

/** The buttons the app registers on its notification categories. */
export const APPROVE = "approve";
export const MERGE = "merge";

const isPayload = (v: unknown): v is Payload =>
  typeof v === "object" && v !== null && typeof (v as Payload).ref === "string" && typeof (v as Payload).since === "string";

/**
 * payloadOf reads the payload from a notification's data: as is (content.data), or from the JSON string a
 * background task gets it in (body or dataString). It is null for anything else.
 */
export function payloadOf(data: unknown): Payload | null {
  if (isPayload(data)) return data;
  if (typeof data !== "object" || data === null) return null;
  for (const key of ["body", "dataString"]) {
    const raw = (data as Record<string, unknown>)[key];
    if (typeof raw !== "string") continue;
    try {
      const parsed: unknown = JSON.parse(raw);
      if (isPayload(parsed)) return parsed;
    } catch {
      // not this one
    }
  }
  return null;
}

export type Route = { kind: "open"; path: string } | { kind: "approve"; ref: string; since: string };

/**
 * routeOf is what pressing a notification does: approve sends the approval of a plan, merge opens the issue with
 * its merge dialog, and the notification itself (or a button that does not fit) opens the issue.
 */
export function routeOf(action: string, p: Payload): Route {
  if (action === APPROVE && p.state === "waiting" && p.reason === "plan") return { kind: "approve", ref: p.ref, since: p.since };
  return { kind: "open", path: issuePath(p.ref, action === MERGE) };
}
