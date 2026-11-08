import { act, render, screen } from "@testing-library/react";
import { useLayoutEffect } from "react";
import {
  LANGUAGE_KEY,
  currentLang,
  defineMessages,
  detectLang,
  readChoice,
  setLanguageChoice,
  useDocumentLang,
  useLanguageChoice,
  useMessages,
} from "@/lib/i18n";

const greeting = defineMessages({
  ja: { hello: "こんにちは", count: (n: number) => `${n}件` },
  en: { hello: "Hello", count: (n) => `${n} ${n === 1 ? "issue" : "issues"}` },
});

function Greeting({ n }: { n: number }) {
  useDocumentLang();
  const t = useMessages(greeting);
  const [choice] = useLanguageChoice();
  return (
    <p>
      {t.hello} {t.count(n)} {choice}
    </p>
  );
}

function setBrowserLanguages(languages: string[]) {
  Object.defineProperty(window.navigator, "languages", { value: languages, configurable: true });
}

afterEach(() => setBrowserLanguages(["ja-JP", "ja"]));

describe("detectLang", () => {
  it("takes Japanese from a Japanese browser", () => {
    expect(detectLang(["ja-JP", "en-US"])).toBe("ja");
  });
  it("takes the first of Japanese and English, so English before Japanese wins", () => {
    expect(detectLang(["fr-FR", "en", "ja"])).toBe("en");
    expect(detectLang(["fr-FR", "ja", "en"])).toBe("ja");
  });
  it("falls back to English when neither is preferred", () => {
    expect(detectLang(["fr-FR"])).toBe("en");
    expect(detectLang([])).toBe("en");
  });
  it("reads a tag in any case and with an underscore", () => {
    expect(detectLang(["JA_jp"])).toBe("ja");
  });
});

describe("the choice", () => {
  it("is auto with nothing stored, and the browser's language is shown", () => {
    setBrowserLanguages(["fr-FR", "en"]);
    expect(readChoice()).toBe("auto");
    expect(currentLang()).toBe("en");
  });

  it("is auto when something unknown is stored", () => {
    localStorage.setItem(LANGUAGE_KEY, "de");
    expect(readChoice()).toBe("auto");
    expect(currentLang()).toBe("ja");
  });

  it("overrides the browser and is kept in the browser", () => {
    setLanguageChoice("en");
    expect(localStorage.getItem(LANGUAGE_KEY)).toBe("en");
    expect(currentLang()).toBe("en");
    setLanguageChoice("auto");
    expect(localStorage.getItem(LANGUAGE_KEY)).toBeNull();
    expect(currentLang()).toBe("ja");
  });
});

describe("a browser that reads its storage but cannot write it", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    setLanguageChoice("auto");
  });

  it("keeps the choice for this page, so the screen still changes language", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("full", "QuotaExceededError");
    });
    render(<Greeting n={1} />);
    act(() => setLanguageChoice("en"));
    expect(readChoice()).toBe("en");
    expect(currentLang()).toBe("en");
    expect(screen.getByText("Hello 1 issue en")).toBeInTheDocument();
    expect(localStorage.getItem(LANGUAGE_KEY)).toBeNull();
  });

  it("follows the stored choice again once it can save one", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("full", "QuotaExceededError");
    });
    setLanguageChoice("en");
    setItem.mockRestore();
    setLanguageChoice("ja");
    expect(localStorage.getItem(LANGUAGE_KEY)).toBe("ja");
    expect(readChoice()).toBe("ja");
    setLanguageChoice("auto");
    expect(readChoice()).toBe("auto");
  });
});

describe("a component", () => {
  it("sets the page's lang before the page is painted, ahead of the layout effects after it", () => {
    document.documentElement.lang = "en";
    const seen: string[] = [];
    function DocumentLang() {
      useDocumentLang();
      return null;
    }
    function Probe() {
      useLayoutEffect(() => {
        seen.push(document.documentElement.lang);
      });
      return null;
    }
    render(
      <>
        <DocumentLang />
        <Probe />
      </>,
    );
    expect(seen[0]).toBe("ja");
  });


  it("is drawn again in the chosen language, without reloading, and the page's lang follows", () => {
    render(<Greeting n={1} />);
    expect(screen.getByText("こんにちは 1件 auto")).toBeInTheDocument();
    expect(document.documentElement.lang).toBe("ja");

    act(() => setLanguageChoice("en"));
    expect(screen.getByText("Hello 1 issue en")).toBeInTheDocument();
    expect(document.documentElement.lang).toBe("en");

    act(() => setLanguageChoice("ja"));
    expect(screen.getByText("こんにちは 1件 ja")).toBeInTheDocument();
  });

  it("follows a choice another tab stored", () => {
    render(<Greeting n={2} />);
    act(() => {
      localStorage.setItem(LANGUAGE_KEY, "en");
      window.dispatchEvent(new StorageEvent("storage", { key: LANGUAGE_KEY, newValue: "en" }));
    });
    expect(screen.getByText("Hello 2 issues en")).toBeInTheDocument();
  });
});

describe("defineMessages", () => {
  it("refuses English text that lacks a key of the Japanese side", () => {
    // @ts-expect-error: en has no "b"
    const m = defineMessages({ ja: { a: "あ", b: "い" }, en: { a: "a" } });
    expect(m.ja.a).toBe("あ");
  });

  it("refuses an English function with other arguments", () => {
    // @ts-expect-error: en takes a string where ja takes a number
    const m = defineMessages({ ja: { f: (n: number) => `${n}` }, en: { f: (s: string) => s } });
    expect(m.ja.f(1)).toBe("1");
  });
});
