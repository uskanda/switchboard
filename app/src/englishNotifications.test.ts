import * as Notifications from "expo-notifications";
import { deviceLang } from "./lib/i18n";
import { handleResponse, setUpNotifications } from "./notifications";

jest.mock("expo-notifications", () => ({
  AndroidImportance: { HIGH: 4 },
  setNotificationHandler: jest.fn(),
  setNotificationChannelAsync: jest.fn(async () => null),
  setNotificationCategoryAsync: jest.fn(async () => null),
  dismissNotificationAsync: jest.fn(async () => undefined),
  scheduleNotificationAsync: jest.fn(async () => "local"),
}));

const N = Notifications as jest.Mocked<typeof Notifications>;
const origin = "http://192.168.0.10:8783";
const plan = { ref: "gh:o/r#20", state: "waiting", reason: "plan", since: "2026-11-02T09:30:00Z" };

let ids = 0;
function press(action: string, data: object): Notifications.NotificationResponse {
  ids += 1;
  return { actionIdentifier: action, notification: { request: { identifier: `en${ids}`, content: { data } } } } as unknown as Notifications.NotificationResponse;
}

let fetchMock: jest.Mock;
beforeEach(() => {
  jest.clearAllMocks();
  fetchMock = jest.fn(async () => new Response(null, { status: 204 }));
  globalThis.fetch = fetchMock as unknown as typeof fetch;
});

/** english makes the app run as on a phone set to English. */
function english() {
  jest.mocked(deviceLang).mockReturnValue("en");
}

const refused = (status: number, code: string, message: string) => async () => new Response(JSON.stringify({ code, message }), { status });

describe("notifications on a phone set to English", () => {
  it("names the channel and the buttons in English", async () => {
    english();
    await setUpNotifications();
    expect(N.setNotificationChannelAsync).toHaveBeenCalledWith("switchboard", { name: "Waiting and done", importance: 4 });
    expect(N.setNotificationCategoryAsync).toHaveBeenCalledWith("plan", [
      { identifier: "approve", buttonTitle: "Approve", options: { opensAppToForeground: false } },
    ]);
    expect(N.setNotificationCategoryAsync).toHaveBeenCalledWith("done-merge", [
      { identifier: "merge", buttonTitle: "Merge", options: { opensAppToForeground: true } },
    ]);
  });

  it.each([
    ["stale", refused(409, "stale", "a newer plan"), "A newer plan was posted, or the issue is no longer waiting for approval. Open it in the app to check."],
    ["unauthenticated", refused(401, "unauthenticated", "log in"), "You need to log in to the inbox. Open the issue in the app to approve it."],
    [
      "unreachable",
      async () => {
        throw new Error("Network request failed");
      },
      "Could not reach the web inbox. Check that you are on your home Wi-Fi.",
    ],
    ["another refusal, in the server's own words", refused(403, "forbidden", "approval is off"), "approval is off"],
  ])("tells a refused approval (%s) in English", async (_name, response, body) => {
    english();
    fetchMock.mockImplementation(response);
    await handleResponse(press("approve", plan), origin);
    expect(N.scheduleNotificationAsync).toHaveBeenCalledWith({
      content: { title: "Could not approve o/r#20", body },
      trigger: { channelId: "switchboard" },
    });
  });
});
