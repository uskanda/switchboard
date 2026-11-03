import * as Notifications from "expo-notifications";
import { useEffect, useState } from "react";
import { View } from "react-native";
import { SafeAreaProvider } from "react-native-safe-area-context";
import { registerDevice } from "./lib/api";
import { loadOrigin, saveOrigin } from "./lib/settings";
import { handleResponse, registerForPush, setUpNotifications } from "./notifications";
import { SettingsScreen } from "./SettingsScreen";
import { WebScreen, type Target } from "./WebScreen";

/**
 * App is the WUI in a WebView (design D1 of add-android-app). Before a URL is saved, and when the user asks, it shows
 * the settings instead. It registers for push notifications with the WUI it shows, and opens what a notification
 * asks for: the issue, or the issue with its merge dialog.
 */
export function App() {
  // undefined while the saved origin is read, null when there is none.
  const [origin, setOrigin] = useState<string | null | undefined>(undefined);
  const [editing, setEditing] = useState(false);
  const [target, setTarget] = useState<Target>({ path: "/", n: 0 });
  const response = Notifications.useLastNotificationResponse();

  useEffect(() => {
    void loadOrigin().then(setOrigin);
    void setUpNotifications().catch(() => undefined);
  }, []);

  useEffect(() => {
    if (!origin) return;
    void registerForPush(origin);
    const sub = Notifications.addPushTokenListener((t) => void registerDevice(origin, String(t.data)));
    return () => sub.remove();
  }, [origin]);

  useEffect(() => {
    if (!origin || !response) return;
    void handleResponse(response, origin, (path) => setTarget((t) => ({ path, n: t.n + 1 })));
  }, [origin, response]);

  let screen;
  if (origin === undefined) {
    screen = <View style={{ flex: 1 }} />;
  } else if (origin === null || editing) {
    screen = (
      <SettingsScreen
        initial={origin}
        onSaved={(o) => {
          void saveOrigin(o);
          setOrigin(o);
          setEditing(false);
        }}
        onCancel={origin ? () => setEditing(false) : undefined}
      />
    );
  } else {
    screen = <WebScreen origin={origin} target={target} onSettings={() => setEditing(true)} />;
  }
  return <SafeAreaProvider>{screen}</SafeAreaProvider>;
}
