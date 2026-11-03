import { useColorScheme } from "react-native";

// The few colors the app's own screens use, from DESIGN.md's tokens; the WUI inside the WebView brings its own.
const light = { surface: "#F6F5F2", raised: "#FFFFFF", text: "#1C1D1F", muted: "#5F636A", outline: "#E2E0DB", field: "#85888E", primary: "#1C1D1F", onPrimary: "#FFFFFF", error: "#B3261E" };
const dark = { surface: "#16181C", raised: "#1F2227", text: "#E8E6E1", muted: "#A3A7AE", outline: "#33373E", field: "#6B7078", primary: "#E8E6E1", onPrimary: "#16181C", error: "#F28B82" };

export type Colors = typeof light;

export function useColors(): Colors {
  return useColorScheme() === "dark" ? dark : light;
}
