import { useEffect, useRef, useState } from "react";
import { BackHandler, Linking, Pressable, StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { WebView } from "react-native-webview";
import { loginGitLab } from "./lib/api";
import { staysInside } from "./lib/url";
import { useColors } from "./theme";

/** Target is the WUI page to show; n changes on each request, so the same page can be asked for twice. */
export type Target = { path: string; n: number };

/**
 * WebScreen shows the WUI at origin in a WebView, below a slim bar whose button leads to the settings. Pages of the
 * WUI stay in the WebView, and so do those of the GitLab people log in on when the login is on (issue #15); any other
 * link opens in the default browser. Back goes back in the WebView while it can.
 */
export function WebScreen({ origin, target, onSettings }: { origin: string; target: Target; onSettings: () => void }) {
  const c = useColors();
  const insets = useSafeAreaInsets();
  // react-native-webview types its props as WebViewProps & P with P = undefined, which TypeScript 6 reduces to
  // never; naming P as an empty object type keeps the props.
  const view = useRef<WebView<object>>(null);
  const canGoBack = useRef(false);
  const [attempt, setAttempt] = useState(0);
  // The failure of one load: the WebView's key it happened under, so a new page or a new attempt clears it.
  const key = `${target.n}-${attempt}`;
  const [failure, setFailure] = useState<{ key: string; text: string } | null>(null);
  const failed = failure?.key === key ? failure.text : null;
  const [gitlab, setGitlab] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    void loginGitLab(origin).then((g) => live && setGitlab(g));
    return () => {
      live = false;
    };
  }, [origin]);

  useEffect(() => {
    const sub = BackHandler.addEventListener("hardwareBackPress", () => {
      if (!canGoBack.current) return false;
      view.current?.goBack();
      return true;
    });
    return () => sub.remove();
  }, []);

  return (
    <View style={[styles.screen, { backgroundColor: c.surface, paddingTop: insets.top, paddingBottom: insets.bottom }]}>
      <View style={[styles.bar, { borderBottomColor: c.outline }]}>
        <Text style={[styles.title, { color: c.text }]}>switchboard</Text>
        <Pressable accessibilityRole="button" onPress={onSettings} style={styles.barButton}>
          <Text style={[styles.label, { color: c.text }]}>接続先</Text>
        </Pressable>
      </View>
      {failed !== null ? (
        <View style={styles.failed}>
          <Text accessibilityRole="alert" style={[styles.heading, { color: c.text }]}>
            受信箱のWeb表示に届きません
          </Text>
          <Text style={[styles.body, { color: c.muted }]}>{origin} に届きませんでした。家のWi-Fiにつながっているか、接続先が合っているかを確かめてください。</Text>
          {failed !== "" && <Text style={[styles.body, styles.mono, { color: c.muted }]}>{failed}</Text>}
          <View style={styles.buttons}>
            <Pressable accessibilityRole="button" onPress={onSettings} style={[styles.button, { borderColor: c.outline }]}>
              <Text style={[styles.label, { color: c.text }]}>接続先を変える</Text>
            </Pressable>
            <Pressable
              accessibilityRole="button"
              onPress={() => setAttempt((a) => a + 1)}
              style={[styles.button, { backgroundColor: c.primary, borderColor: c.primary }]}
            >
              <Text style={[styles.label, { color: c.onPrimary }]}>もう一度読む</Text>
            </Pressable>
          </View>
        </View>
      ) : (
        <WebView<object>
          key={key}
          ref={view}
          source={{ uri: origin + target.path }}
          style={{ backgroundColor: c.surface }}
          setSupportMultipleWindows={false}
          onShouldStartLoadWithRequest={(req) => {
            if (staysInside(origin, gitlab, req.url) || req.url.startsWith("about:")) return true;
            void Linking.openURL(req.url);
            return false;
          }}
          onNavigationStateChange={(nav) => {
            canGoBack.current = nav.canGoBack;
          }}
          onError={(e) => setFailure({ key, text: e.nativeEvent.description ?? "" })}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  bar: { minHeight: 44, flexDirection: "row", alignItems: "center", justifyContent: "space-between", paddingLeft: 16, borderBottomWidth: 1 },
  title: { fontSize: 15, fontWeight: "600" },
  barButton: { minHeight: 44, minWidth: 44, paddingHorizontal: 16, alignItems: "center", justifyContent: "center" },
  failed: { flex: 1, padding: 16, gap: 12 },
  heading: { fontSize: 20, fontWeight: "600", lineHeight: 26 },
  body: { fontSize: 15, lineHeight: 24 },
  mono: { fontFamily: "monospace", fontSize: 13 },
  buttons: { flexDirection: "row", flexWrap: "wrap", justifyContent: "flex-end", gap: 8 },
  button: { minHeight: 44, paddingHorizontal: 16, borderWidth: 1, borderRadius: 6, alignItems: "center", justifyContent: "center" },
  label: { fontSize: 15, fontWeight: "600" },
});
