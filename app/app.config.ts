import type { ExpoConfig } from "expo/config";

// The Android app: the web inbox in a WebView, with push notifications (add-android-app). google-services.json is
// the user's own Firebase app's, named by SWITCHBOARD_APP_GOOGLE_SERVICES at build time and never committed.
const config: ExpoConfig = {
  name: "switchboard",
  slug: "switchboard",
  version: "0.1.0",
  userInterfaceStyle: "automatic",
  android: {
    package: "io.github.uskanda.switchboard",
    googleServicesFile: process.env.SWITCHBOARD_APP_GOOGLE_SERVICES,
  },
  plugins: [
    // The web inbox is plain HTTP on the LAN (ADR-036).
    ["expo-build-properties", { android: { usesCleartextTraffic: true } }],
    ["expo-notifications", { defaultChannel: "switchboard" }],
    "./plugins/withReleaseSigning",
  ],
};

export default config;
