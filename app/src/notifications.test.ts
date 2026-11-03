import * as Notifications from "expo-notifications";
import { handleResponse, registerForPush, setUpNotifications } from "./notifications";

jest.mock("expo-notifications", () => ({
  AndroidImportance: { HIGH: 4 },
  setNotificationHandler: jest.fn(),
  setNotificationChannelAsync: jest.fn(async () => null),
  setNotificationCategoryAsync: jest.fn(async () => null),
  getPermissionsAsync: jest.fn(),
  requestPermissionsAsync: jest.fn(),
  getDevicePushTokenAsync: jest.fn(),
  dismissNotificationAsync: jest.fn(async () => undefined),
  scheduleNotificationAsync: jest.fn(async () => "local"),
}));

const N = Notifications as jest.Mocked<typeof Notifications>;
const origin = "http://192.168.0.10:8783";
const DEFAULT = "expo.modules.notifications.actions.DEFAULT";
const plan = { ref: "gh:o/r#20", state: "waiting", reason: "plan", since: "2026-11-02T09:30:00Z" };
const done = { ref: "gh:o/r#12", state: "done", since: "2026-11-02T09:30:00Z", change_number: 9, change_sha: "abc" };

let ids = 0;
function press(action: string, data: object): Notifications.NotificationResponse {
  ids += 1;
  return { actionIdentifier: action, notification: { request: { identifier: `n${ids}`, content: { data } } } } as unknown as Notifications.NotificationResponse;
}

let fetchMock: jest.Mock;
beforeEach(() => {
  jest.clearAllMocks();
  fetchMock = jest.fn(async () => new Response(null, { status: 204 }));
  globalThis.fetch = fetchMock as unknown as typeof fetch;
});

describe("setUpNotifications", () => {
  it("gives a plan 承認 without opening the app, and a done issue マージ that opens it", async () => {
    await setUpNotifications();
    expect(N.setNotificationChannelAsync).toHaveBeenCalledWith("switchboard", expect.anything());
    expect(N.setNotificationCategoryAsync).toHaveBeenCalledWith("plan", [
      { identifier: "approve", buttonTitle: "承認", options: { opensAppToForeground: false } },
    ]);
    expect(N.setNotificationCategoryAsync).toHaveBeenCalledWith("done-merge", [
      { identifier: "merge", buttonTitle: "マージ", options: { opensAppToForeground: true } },
    ]);
  });
});

describe("registerForPush", () => {
  it("registers the FCM token with the WUI, with its Origin", async () => {
    N.getPermissionsAsync.mockResolvedValue({ granted: true, canAskAgain: true } as never);
    N.getDevicePushTokenAsync.mockResolvedValue({ type: "android", data: "fcm-1" });
    await expect(registerForPush(origin)).resolves.toEqual({ ok: true });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${origin}/api/devices`);
    expect(init.headers.Origin).toBe(origin);
    expect(JSON.parse(init.body)).toEqual({ token: "fcm-1" });
  });

  it("registers nothing when notifications are refused", async () => {
    N.getPermissionsAsync.mockResolvedValue({ granted: false, canAskAgain: true } as never);
    N.requestPermissionsAsync.mockResolvedValue({ granted: false } as never);
    await expect(registerForPush(origin)).resolves.toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("never throws, so the WUI is used the same without a token", async () => {
    N.getPermissionsAsync.mockResolvedValue({ granted: true } as never);
    N.getDevicePushTokenAsync.mockRejectedValue(new Error("no google-services"));
    await expect(registerForPush(origin)).resolves.toMatchObject({ ok: false });
  });
});

describe("handleResponse", () => {
  it("opens the issue when the notification itself is pressed", async () => {
    const open = jest.fn();
    await handleResponse(press(DEFAULT, plan), origin, open);
    expect(open).toHaveBeenCalledWith("/issue/gh%3Ao%2Fr%2320");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("opens the merge dialog on マージ, and merges nothing itself", async () => {
    const open = jest.fn();
    await handleResponse(press("merge", done), origin, open);
    expect(open).toHaveBeenCalledWith("/issue/gh%3Ao%2Fr%2312?action=merge");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("approves on 承認 and removes the notification", async () => {
    const r = press("approve", plan);
    await handleResponse(r, origin);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${origin}/api/approve`);
    expect(init.headers.Origin).toBe(origin);
    expect(JSON.parse(init.body)).toEqual({ ref: "gh:o/r#20", since: "2026-11-02T09:30:00Z" });
    expect(N.dismissNotificationAsync).toHaveBeenCalledWith(r.notification.request.identifier);
  });

  it("tells a refused approval in a local notification", async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ code: "stale", message: "a newer plan" }), { status: 409 }));
    await handleResponse(press("approve", plan), origin);
    expect(N.dismissNotificationAsync).not.toHaveBeenCalled();
    const req = N.scheduleNotificationAsync.mock.calls[0][0];
    expect(req.content.title).toBe("o/r#20 を承認しませんでした");
    expect(req.content.body).toContain("計画が出し直された");
  });

  it("acts once on a press that reaches it twice", async () => {
    const r = press("approve", plan);
    await handleResponse(r, origin);
    await handleResponse(r, origin, jest.fn());
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("leaves an opening press to the app when it runs without a screen", async () => {
    const r = press(DEFAULT, plan);
    await handleResponse(r, origin);
    const open = jest.fn();
    await handleResponse(r, origin, open);
    expect(open).toHaveBeenCalledWith("/issue/gh%3Ao%2Fr%2320");
  });

  it("ignores a notification that is not the inbox service's", async () => {
    const open = jest.fn();
    await handleResponse(press(DEFAULT, { something: "else" }), origin, open);
    expect(open).not.toHaveBeenCalled();
  });
});
