import { I18nManager } from "react-native";
import { deviceLang } from "./i18n";

// jest.setup.ts makes deviceLang Japanese for the other tests; this file tests the real one.
jest.unmock("./i18n");

function intlLocale(locale: string) {
  jest.spyOn(Intl.DateTimeFormat.prototype, "resolvedOptions").mockReturnValue({ locale } as Intl.ResolvedDateTimeFormatOptions);
}

function nativeLocale(localeIdentifier: string | undefined) {
  jest.spyOn(I18nManager, "getConstants").mockReturnValue({ isRTL: false, doLeftAndRightSwapInRTL: true, localeIdentifier });
}

afterEach(() => jest.restoreAllMocks());

describe("deviceLang", () => {
  it("is Japanese on a phone set to Japanese", () => {
    intlLocale("ja-JP");
    nativeLocale("en_US");
    expect(deviceLang()).toBe("ja");
  });

  it.each(["en-US", "en-GB", "fr-FR", "zh-Hans-CN"])("is English on a phone set to %s", (locale) => {
    intlLocale(locale);
    nativeLocale("ja_JP");
    expect(deviceLang()).toBe("en");
  });

  it("reads React Native's locale when Intl gives none", () => {
    intlLocale("");
    nativeLocale("ja_JP");
    expect(deviceLang()).toBe("ja");
  });

  it("is English when neither gives a locale", () => {
    intlLocale("");
    nativeLocale(undefined);
    expect(deviceLang()).toBe("en");
  });
});
