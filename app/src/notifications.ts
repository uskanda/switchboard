import * as Notifications from "expo-notifications";
import { approve, registerDevice, type Result } from "./lib/api";
import { claim } from "./lib/claims";
import { APPROVE, MERGE, payloadOf, routeOf } from "./lib/payload";

// The channel and the categories the inbox service's notifications name (internal/push): a plan waiting for
// approval gets 承認, which approves without opening the app; a done issue with its change request gets マージ, which
// opens the app on the merge dialog (Q12 of add-android-app).
export const CHANNEL = "switchboard";
export const CATEGORY_PLAN = "plan";
export const CATEGORY_MERGE = "done-merge";

/**
 * setUpNotifications creates the channel and the categories, and shows notifications that arrive in the foreground.
 * With the WUI's login on, 承認 opens the app on the plan instead of approving in the background (issue #146).
 */
export async function setUpNotifications(loginOn = false): Promise<void> {
  Notifications.setNotificationHandler({
    handleNotification: async () => ({ shouldShowBanner: true, shouldShowList: true, shouldPlaySound: true, shouldSetBadge: false }),
  });
  await Notifications.setNotificationChannelAsync(CHANNEL, { name: "待ちと完了", importance: Notifications.AndroidImportance.HIGH });
  await Notifications.setNotificationCategoryAsync(CATEGORY_PLAN, [
    { identifier: APPROVE, buttonTitle: "承認", options: { opensAppToForeground: loginOn } },
  ]);
  await Notifications.setNotificationCategoryAsync(CATEGORY_MERGE, [
    { identifier: MERGE, buttonTitle: "マージ", options: { opensAppToForeground: true } },
  ]);
}

/**
 * registerForPush asks for the permission once and registers the device's FCM token with the WUI at origin. It is
 * null when notifications are not allowed. It never throws: the WUI is used the same without notifications.
 */
export async function registerForPush(origin: string): Promise<Result | null> {
  try {
    const perm = await Notifications.getPermissionsAsync();
    let granted = perm.granted;
    if (!granted && perm.canAskAgain) granted = (await Notifications.requestPermissionsAsync()).granted;
    if (!granted) return null;
    const token = await Notifications.getDevicePushTokenAsync();
    return await registerDevice(origin, String(token.data));
  } catch (err) {
    return { ok: false, code: "unregistered", message: err instanceof Error ? err.message : String(err) };
  }
}

function refusal(r: Exclude<Result, { ok: true }>): string {
  switch (r.code) {
    case "stale":
      return "計画が出し直されたか、もう承認を待っていません。アプリで開いて確かめてください。";
    case "unauthenticated":
      return "受信箱にログインが要ります。アプリで開いて承認してください。";
    case "unreachable":
      return "受信箱のWeb表示に届きませんでした。家のWi-Fiにつながっているか確かめてください。";
  }
  return r.message;
}

/**
 * handleResponse does what a press on a notification asks: open opens the issue (open is not given in the background
 * task, which has no screen), approve sends the approval and removes the notification, and a refused approval is
 * told in a local notification. Each press is acted on once.
 */
export async function handleResponse(response: Notifications.NotificationResponse, origin: string, open?: (path: string) => void, loginOn = false) {
  const request = response.notification.request;
  const p = payloadOf(request.content.data);
  if (!p) return;
  const route = routeOf(response.actionIdentifier, p, loginOn);
  if (route.kind === "open" && !open) return;
  if (!(await claim(`${request.identifier}:${response.actionIdentifier}`))) return;
  if (route.kind === "open") {
    open?.(route.path);
    return;
  }
  const r = await approve(origin, route.ref, route.since);
  if (r.ok) {
    await Notifications.dismissNotificationAsync(request.identifier);
    return;
  }
  await Notifications.scheduleNotificationAsync({
    content: { title: `${route.ref.replace(/^g[hl]:/, "")} を承認しませんでした`, body: refusal(r) },
    trigger: { channelId: CHANNEL },
  });
}
