import { approve, registerDevice } from "./api";

const origin = "http://192.168.0.10:8783";

function fakeFetch(status: number, body?: unknown) {
  return jest.fn<Promise<Response>, [string, RequestInit?]>(async () => new Response(body === undefined ? null : JSON.stringify(body), { status }));
}

describe("the app's POSTs", () => {
  it("register the token as JSON with the WUI's own Origin", async () => {
    const f = fakeFetch(204);
    await expect(registerDevice(origin, "fcm-1", f)).resolves.toEqual({ ok: true });
    const [url, init] = f.mock.calls[0];
    expect(url).toBe("http://192.168.0.10:8783/api/devices");
    expect(init?.method).toBe("POST");
    expect(init?.headers).toEqual({ "Content-Type": "application/json", Origin: origin });
    expect(JSON.parse(String(init?.body))).toEqual({ token: "fcm-1" });
  });

  it("send an approval with the start of the wait", async () => {
    const f = fakeFetch(204);
    await approve(origin, "gh:o/r#20", "2026-11-02T09:30:00Z", f);
    const [url, init] = f.mock.calls[0];
    expect(url).toBe("http://192.168.0.10:8783/api/approve");
    expect(JSON.parse(String(init?.body))).toEqual({ ref: "gh:o/r#20", since: "2026-11-02T09:30:00Z" });
  });

  it("return the refusal's code and message", async () => {
    const f = fakeFetch(409, { code: "stale", message: "a newer plan" });
    await expect(approve(origin, "gh:o/r#20", "2026-11-02T09:30:00Z", f)).resolves.toEqual({ ok: false, code: "stale", message: "a newer plan" });
  });

  it("return unreachable when no answer comes", async () => {
    const f = jest.fn(async () => {
      throw new TypeError("Network request failed");
    });
    await expect(registerDevice(origin, "fcm-1", f)).resolves.toEqual({ ok: false, code: "unreachable", message: "Network request failed" });
  });
});
