import mockAsyncStorage from "@react-native-async-storage/async-storage/jest/async-storage-mock";

jest.mock("@react-native-async-storage/async-storage", () => mockAsyncStorage);

jest.mock("react-native-safe-area-context", () => require("react-native-safe-area-context/jest/mock").default);

// The app's own text is Japanese in the tests, as on a phone set to Japanese. A test of English makes deviceLang
// return "en"; src/lib/i18n.test.ts tests the real one.
jest.mock("./src/lib/i18n", () => ({ ...jest.requireActual("./src/lib/i18n"), deviceLang: jest.fn(() => "ja") }));
