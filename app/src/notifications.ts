import * as Notifications from "expo-notifications";
import { approve, registerDevice, type Result } from "./lib/api";
import { claim } from "./lib/claims";
import { deviceLang, messages, type Messages } from "./lib/i18n";
import { APPROVE, MERGE, payloadOf, routeOf } from "./lib/payload";

// The channel and the categories the inbox service's notifications name (internal/push): a plan waiting for
// approval gets Approve, which approves without opening the app; a done issue with its change request gets Merge,
// which opens the app on the merge dialog (Q12 of add-android-app). Their names are in the phone's language, set
// again on each start of the app.
export const CHANNEL = "switchboard";
export const CATEGORY_PLAN = "plan";
export const CATEGORY_MERGE = "done-merge";

/**
 * setUpNotifications creates the channel and the categories, and shows notifications that arrive in the foreground.
 * With the WUI's login on, Approve opens the app on the plan instead of approving in the background (issue #146).
 */
export async function setUpNotifications(loginOn = false): Promise<void> {
  const t = messages[deviceLang()].notification;
  Notifications.setNotificationHandler({
    handleNotification: async () => ({ shouldShowBanner: true, shouldShowList: true, shouldPlaySound: true, shouldSetBadge: false }),
  });
  await Notifications.setNotificationChannelAsync(CHANNEL, { name: t.channel, importance: Notifications.AndroidImportance.HIGH });
  await Notifications.setNotificationCategoryAsync(CATEGORY_PLAN, [
    { identifier: APPROVE, buttonTitle: t.approve, options: { opensAppToForeground: loginOn } },
  ]);
  await Notifications.setNotificationCategoryAsync(CATEGORY_MERGE, [
    { identifier: MERGE, buttonTitle: t.merge, options: { opensAppToForeground: true } },
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

/** refusal tells why an approval was refused: in t for the codes the app knows, else in the server's own words. */
function refusal(r: Exclude<Result, { ok: true }>, t: Messages["notification"]): string {
  switch (r.code) {
    case "stale":
      return t.stale;
    case "unauthenticated":
      return t.unauthenticated;
    case "unreachable":
      return t.unreachable;
  }
  return r.message;
}

/**
 * handleResponse does what a press on a notification asks: open opens the issue (open is not given in the background
 * task, which has no screen), approve sends the approval and removes the notification, and a refused approval is
 * told in a local notification, in the phone's language. Each press is acted on once.
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
  const t = messages[deviceLang()].notification;
  await Notifications.scheduleNotificationAsync({
    content: { title: t.notApproved(route.ref.replace(/^g[hl]:/, "")), body: refusal(r, t) },
    trigger: { channelId: CHANNEL },
  });
}
