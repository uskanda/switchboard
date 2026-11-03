import * as Notifications from "expo-notifications";
import * as TaskManager from "expo-task-manager";
import { loadOrigin } from "./lib/settings";
import { handleResponse } from "./notifications";

// On Android, a press on a button that does not open the app (承認) runs this task, even when the app is not
// running. It must be defined at the top level of a module the entry point loads.
export const TASK = "switchboard-notification-response";

TaskManager.defineTask<Notifications.NotificationTaskPayload>(TASK, async ({ data, error }) => {
  if (error || !data || !("actionIdentifier" in data)) return;
  const origin = await loadOrigin();
  if (origin) await handleResponse(data, origin);
});

Notifications.registerTaskAsync(TASK).catch(() => {
  // Without the task, 承認 still works while the app runs.
});
