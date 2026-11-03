import AsyncStorage from "@react-native-async-storage/async-storage";
import { parseWebURL } from "./url";

// The WUI's origin, as the user entered it on the settings screen. It is not a secret.
const KEY = "switchboard.web-origin";

/** loadOrigin is the saved origin, or null before one is saved. */
export async function loadOrigin(): Promise<string | null> {
  const raw = await AsyncStorage.getItem(KEY);
  if (!raw) return null;
  const p = parseWebURL(raw);
  return p.ok ? p.origin : null;
}

export const saveOrigin = (origin: string) => AsyncStorage.setItem(KEY, origin);
