import AsyncStorage from "@react-native-async-storage/async-storage";
import * as Notifications from "expo-notifications";
import * as TaskManager from "expo-task-manager";
import { deviceLang } from "./lib/i18n";
import { TASK } from "./backgroundTask";

type Task = (body: { data: unknown; error: unknown }) => Promise<void>;

// The mock keeps the tasks the module defines, since the module defines its task on import.
jest.mock("expo-task-manager", () => {
  const tasks: Record<string, Task> = {};
  return {
    tasks,
    defineTask: (name: string, task: Task) => {
      tasks[name] = task;
    },
  };
});

jest.mock("expo-notifications", () => ({
  AndroidImportance: { HIGH: 4 },
  registerTaskAsync: jest.fn(async () => null),
  dismissNotificationAsync: jest.fn(async () => undefined),
  scheduleNotificationAsync: jest.fn(async () => "local"),
}));

const N = Notifications as jest.Mocked<typeof Notifications>;
const origin = "http://192.168.0.10:8783";
const plan = { ref: "gh:o/r#20", state: "waiting", reason: "plan", since: "2026-11-02T09:30:00Z" };

describe("the background task on a phone set to English", () => {
  it("tells a refused approval in English while the app is not running", async () => {
    jest.mocked(deviceLang).mockReturnValue("en");
    await AsyncStorage.setItem("switchboard.web-origin", origin);
    globalThis.fetch = jest.fn(async () => new Response(JSON.stringify({ code: "stale", message: "a newer plan" }), { status: 409 })) as unknown as typeof fetch;
    const data = { actionIdentifier: "approve", notification: { request: { identifier: "bg-1", content: { data: plan } } } };
    const tasks = (TaskManager as unknown as { tasks: Record<string, Task> }).tasks;
    await tasks[TASK]({ data, error: null });
    expect(N.scheduleNotificationAsync).toHaveBeenCalledWith(
      expect.objectContaining({ content: expect.objectContaining({ title: "Could not approve o/r#20" }) }),
    );
  });
});
