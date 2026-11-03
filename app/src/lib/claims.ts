import AsyncStorage from "@react-native-async-storage/async-storage";

// A press on a notification can reach the app twice: through its listener while it runs, and through the background
// task. claim lets one of them act on it.
const KEY = "switchboard.handled-responses";
const KEEP = 50;
const seen = new Set<string>();

/** claim returns true the first time it is given key, in this process or a past one; false after that. */
export async function claim(key: string): Promise<boolean> {
  if (seen.has(key)) return false;
  seen.add(key);
  let list: string[];
  try {
    list = JSON.parse((await AsyncStorage.getItem(KEY)) ?? "[]");
  } catch {
    list = [];
  }
  if (list.includes(key)) return false;
  await AsyncStorage.setItem(KEY, JSON.stringify([...list, key].slice(-KEEP)));
  return true;
}
