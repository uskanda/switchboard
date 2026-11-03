// The POSTs the app sends to switchboard-web itself, outside the WebView: a token registration and an approval.
// React Native's fetch sets no Origin, and switchboard-web takes a POST only with an Origin of its listen address,
// so each one names the WUI's origin (design D3 of add-android-app).

export type Result = { ok: true } | { ok: false; code: string; message: string };

type Fetch = (url: string, init?: RequestInit) => Promise<Response>;

async function postJSON(origin: string, path: string, body: unknown, f: Fetch): Promise<Result> {
  let res: Response;
  try {
    res = await f(origin + path, {
      method: "POST",
      headers: { "Content-Type": "application/json", Origin: origin },
      body: JSON.stringify(body),
    });
  } catch (err) {
    return { ok: false, code: "unreachable", message: err instanceof Error ? err.message : String(err) };
  }
  if (res.ok) return { ok: true };
  const e = (await res.json().catch(() => ({}))) as { code?: string; message?: string };
  return { ok: false, code: e.code ?? "unknown", message: e.message ?? `${res.status}` };
}

/** registerDevice adds the app's FCM token to the devices the inbox service sends push notifications to. */
export const registerDevice = (origin: string, token: string, f: Fetch = fetch) => postJSON(origin, "/api/devices", { token }, f);

/** approve posts the approve word on an issue still waiting on the plan whose wait started at since. */
export const approve = (origin: string, ref: string, since: string, f: Fetch = fetch) =>
  postJSON(origin, "/api/approve", { ref, since }, f);
