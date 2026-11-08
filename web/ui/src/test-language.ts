// Runs before test-setup.ts. jsdom reports an English browser; the tests read the inbox in Japanese, as most were
// written in it, unless a test chooses English with setLanguageChoice("en") from @/lib/i18n. Defining the properties
// on the navigator itself keeps them through vi.unstubAllGlobals, vi.restoreAllMocks and localStorage.clear.
Object.defineProperty(window.navigator, "language", { value: "ja-JP", configurable: true });
Object.defineProperty(window.navigator, "languages", { value: ["ja-JP", "ja"], configurable: true });
