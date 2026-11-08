import { I18nManager } from "react-native";

// The language of the app's own text (ADR-065): the settings screen, the bar and the error over the WebView, and the
// notifications the app sets up or posts itself. It follows the phone: Japanese on a phone set to Japanese, English
// on any other. The WUI in the WebView picks its own language from the WebView's languages, so the app gives it none.

/** Lang is a language the app shows its own text in. */
export type Lang = "ja" | "en";

/** langOf is the language for a locale such as "ja-JP" or "ja_JP": Japanese for Japanese, English for any other. */
export function langOf(locale: string): Lang {
  return locale.trim().toLowerCase().split(/[-_]/)[0] === "ja" ? "ja" : "en";
}

function intlLocale(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().locale || undefined;
  } catch {
    return undefined;
  }
}

function nativeLocale(): string | undefined {
  try {
    return I18nManager.getConstants().localeIdentifier || undefined;
  } catch {
    return undefined;
  }
}

/**
 * deviceLang is the phone's language as the app uses it: the locale of the standard Intl, or React Native's
 * I18nManager when Intl gives none. The screens and the background task both call it, each time they need text.
 */
export function deviceLang(): Lang {
  return langOf(intlLocale() ?? nativeLocale() ?? "");
}

/** Messages is the app's own text in one language. */
export type Messages = {
  /** The screen that asks for the WUI's URL. */
  settings: { heading: string; help: string; urlLabel: string; cancel: string; save: string };
  /** badURL is the error of a URL that is not an absolute http or https one. */
  badURL: string;
  /** The bar over the WebView, and what replaces the WebView when the WUI cannot be reached. */
  web: { address: string; unreachable: string; unreachableHelp: (origin: string) => string; changeAddress: string; reload: string };
  /** The channel, the buttons, and the local notification of a refused approval. */
  notification: {
    channel: string;
    approve: string;
    merge: string;
    notApproved: (ref: string) => string;
    stale: string;
    unauthenticated: string;
    unreachable: string;
  };
};

/** messages is the app's own text in each language. */
export const messages: Record<Lang, Messages> = {
  ja: {
    settings: {
      heading: "受信箱のWeb表示の場所",
      help: "web.yaml の listen と同じIPアドレスとポートを、http:// を付けて入力します。家のWi-Fiの中からだけ届きます。",
      urlLabel: "Web表示のURL",
      cancel: "やめる",
      save: "保存する",
    },
    badURL: "http か https で始まる絶対URLを入力してください（例：http://192.168.0.10:8783）。",
    web: {
      address: "接続先",
      unreachable: "受信箱のWeb表示に届きません",
      unreachableHelp: (origin) => `${origin} に届きませんでした。家のWi-Fiにつながっているか、接続先が合っているかを確かめてください。`,
      changeAddress: "接続先を変える",
      reload: "もう一度読む",
    },
    notification: {
      channel: "待ちと完了",
      approve: "承認",
      merge: "マージ",
      notApproved: (ref) => `${ref} を承認しませんでした`,
      stale: "計画が出し直されたか、もう承認を待っていません。アプリで開いて確かめてください。",
      unauthenticated: "受信箱にログインが要ります。アプリで開いて承認してください。",
      unreachable: "受信箱のWeb表示に届きませんでした。家のWi-Fiにつながっているか確かめてください。",
    },
  },
  en: {
    settings: {
      heading: "Web inbox address",
      help: "Enter the IP address and port set as listen in web.yaml, starting with http://. The web inbox is reachable only from your home Wi-Fi.",
      urlLabel: "Web inbox URL",
      cancel: "Cancel",
      save: "Save",
    },
    badURL: "Enter an absolute URL that starts with http or https (for example, http://192.168.0.10:8783).",
    web: {
      address: "Address",
      unreachable: "Cannot reach the web inbox",
      unreachableHelp: (origin) => `Could not reach ${origin}. Check that you are on your home Wi-Fi and that the address is right.`,
      changeAddress: "Change address",
      reload: "Reload",
    },
    notification: {
      channel: "Waiting and done",
      approve: "Approve",
      merge: "Merge",
      notApproved: (ref) => `Could not approve ${ref}`,
      stale: "A newer plan was posted, or the issue is no longer waiting for approval. Open it in the app to check.",
      unauthenticated: "You need to log in to the inbox. Open the issue in the app to approve it.",
      unreachable: "Could not reach the web inbox. Check that you are on your home Wi-Fi.",
    },
  },
};
