import { useLayoutEffect, useSyncExternalStore } from "react";

/**
 * The language the web inbox is shown in (ADR-065). The viewer chooses it on the settings page; until they do, it
 * follows the browser. The choice stays in this browser and is never sent to the inbox service.
 */

/** Lang is a language the web inbox is shown in. */
export type Lang = "ja" | "en";

/** LangChoice is what the viewer chose: a language, or "auto" to follow the browser. */
export type LangChoice = Lang | "auto";

/** Where the browser remembers the chosen language. Nothing stored (or anything else) is "auto". */
export const LANGUAGE_KEY = "switchboard.language";

/**
 * detectLang is the language of the first of the browser's preferred languages that is Japanese or English, and
 * English when none is.
 */
export function detectLang(languages: readonly string[]): Lang {
  for (const tag of languages) {
    const primary = tag.trim().toLowerCase().split(/[-_]/)[0];
    if (primary === "ja" || primary === "en") return primary;
  }
  return "en";
}

function browserLanguages(): readonly string[] {
  if (typeof navigator === "undefined") return [];
  if (navigator.languages && navigator.languages.length > 0) return navigator.languages;
  return navigator.language ? [navigator.language] : [];
}

// A browser that cannot save the choice (no storage, or storage that is full) keeps it for this page only.
let unsaved: LangChoice | null = null;

/** readChoice is the choice stored in this browser: "ja", "en", or "auto" when there is none. */
export function readChoice(): LangChoice {
  if (unsaved !== null) return unsaved;
  try {
    const v = window.localStorage.getItem(LANGUAGE_KEY);
    return v === "ja" || v === "en" ? v : "auto";
  } catch {
    return "auto";
  }
}

/** currentLang is the language the inbox is shown in now, for code outside React: the choice, or the browser's. */
export function currentLang(): Lang {
  const choice = readChoice();
  return choice === "auto" ? detectLang(browserLanguages()) : choice;
}

const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  // Another tab that changes the choice changes it here too; so does a change of the browser's languages.
  const onStorage = (e: StorageEvent) => {
    if (e.key === null || e.key === LANGUAGE_KEY) listener();
  };
  window.addEventListener("storage", onStorage);
  window.addEventListener("languagechange", listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
    window.removeEventListener("languagechange", listener);
  };
}

/** setLanguageChoice keeps the choice in this browser and shows every screen of this tab in it at once. */
export function setLanguageChoice(choice: LangChoice) {
  try {
    if (choice === "auto") window.localStorage.removeItem(LANGUAGE_KEY);
    else window.localStorage.setItem(LANGUAGE_KEY, choice);
    unsaved = null;
  } catch {
    unsaved = choice;
  }
  for (const l of [...listeners]) l();
}

/** useLang is the language the inbox is shown in; a component that uses it is drawn again when it changes. */
export function useLang(): Lang {
  return useSyncExternalStore(subscribe, currentLang, () => "en");
}

/** useLanguageChoice is the viewer's choice and the function that changes it. */
export function useLanguageChoice() {
  const choice = useSyncExternalStore(subscribe, readChoice, (): LangChoice => "auto");
  return [choice, setLanguageChoice] as const;
}

/**
 * useDocumentLang keeps the lang attribute of the page on the language shown. It sets it before the page is painted,
 * so no frame shows the text under the other language's lang (and its fonts).
 */
export function useDocumentLang() {
  const lang = useLang();
  useLayoutEffect(() => {
    document.documentElement.lang = lang;
  }, [lang]);
}

/** Messages is the text of one part of the inbox in both languages. */
export type Messages<T> = { ja: T; en: T };

/**
 * defineMessages declares the text of one part of the inbox. The English side must have exactly the keys of the
 * Japanese side, and its functions the same arguments, or the type check fails: no text is left in one language only.
 * A value is a string, or a function of numbers, names or elements that returns a string or an element, so each
 * language orders the words of a sentence its own way.
 */
export function defineMessages<T>(messages: { ja: T; en: NoInfer<T> }): Messages<T> {
  return messages;
}

/** useMessages is the side of messages in the language shown. */
export function useMessages<T>(messages: Messages<T>): T {
  return messages[useLang()];
}

/** plural is the English word for n of something: one when n is 1, other otherwise. */
export const plural = (n: number, one: string, other: string) => (n === 1 ? one : other);

/** errorText is the message of an error, for text that shows it after words of its own. */
export const errorText = (err: unknown) => (err instanceof Error ? err.message : String(err));
