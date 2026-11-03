import { useState } from "react";
import { Pressable, StyleSheet, Text, TextInput, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { parseWebURL } from "./lib/url";
import { useColors } from "./theme";

/**
 * SettingsScreen asks for the WUI's URL: on the first start, and whenever the user changes it. Only an http or
 * https URL is kept, and only its origin.
 */
export function SettingsScreen({ initial, onSaved, onCancel }: { initial: string | null; onSaved: (origin: string) => void; onCancel?: () => void }) {
  const c = useColors();
  const insets = useSafeAreaInsets();
  const [text, setText] = useState(initial ?? "");
  const [error, setError] = useState<string | null>(null);

  function save() {
    const p = parseWebURL(text);
    if (!p.ok) {
      setError(p.error);
      return;
    }
    onSaved(p.origin);
  }

  return (
    <View style={[styles.screen, { backgroundColor: c.surface, paddingTop: insets.top + 24, paddingBottom: insets.bottom + 24 }]}>
      <Text accessibilityRole="header" style={[styles.heading, { color: c.text }]}>
        受信箱のWeb表示の場所
      </Text>
      <Text style={[styles.body, { color: c.muted }]}>web.yaml の listen と同じIPアドレスとポートを、http:// を付けて入力します。家のWi-Fiの中からだけ届きます。</Text>
      <TextInput
        accessibilityLabel="Web表示のURL"
        value={text}
        onChangeText={(t) => {
          setText(t);
          setError(null);
        }}
        onSubmitEditing={save}
        placeholder="http://192.168.0.10:8783"
        placeholderTextColor={c.muted}
        autoCapitalize="none"
        autoCorrect={false}
        keyboardType="url"
        style={[styles.input, { color: c.text, borderColor: error ? c.error : c.field, backgroundColor: c.raised }]}
      />
      {error && (
        <Text accessibilityRole="alert" style={[styles.body, { color: c.error }]}>
          {error}
        </Text>
      )}
      <View style={styles.buttons}>
        {onCancel && (
          <Pressable accessibilityRole="button" onPress={onCancel} style={[styles.button, { borderColor: c.outline }]}>
            <Text style={[styles.label, { color: c.text }]}>やめる</Text>
          </Pressable>
        )}
        <Pressable accessibilityRole="button" onPress={save} style={[styles.button, { backgroundColor: c.primary, borderColor: c.primary }]}>
          <Text style={[styles.label, { color: c.onPrimary }]}>保存する</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, paddingHorizontal: 16, gap: 12 },
  heading: { fontSize: 20, fontWeight: "600", lineHeight: 26 },
  body: { fontSize: 15, lineHeight: 24 },
  input: { minHeight: 44, borderWidth: 1, borderRadius: 6, paddingHorizontal: 12, fontSize: 15, fontFamily: "monospace" },
  buttons: { flexDirection: "row", justifyContent: "flex-end", gap: 8 },
  button: { minHeight: 44, minWidth: 88, paddingHorizontal: 16, borderWidth: 1, borderRadius: 6, alignItems: "center", justifyContent: "center" },
  label: { fontSize: 15, fontWeight: "600" },
});
