import AsyncStorage from "@react-native-async-storage/async-storage";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react-native";
import { BackHandler, Linking } from "react-native";
import { App } from "./App";

type WebProps = {
  source: { uri: string };
  onShouldStartLoadWithRequest: (req: { url: string }) => boolean;
  onNavigationStateChange: (nav: { canGoBack: boolean }) => void;
  onError: (e: { nativeEvent: { description?: string } }) => void;
};

let mockWeb: WebProps | null = null;
const mockGoBack = jest.fn();
let mockResponse: unknown = null;

jest.mock("react-native-webview", () => {
  const React = require("react");
  const { View } = require("react-native");
  const WebView = React.forwardRef((props: WebProps, ref: unknown) => {
    React.useImperativeHandle(ref, () => ({ goBack: mockGoBack }));
    mockWeb = props;
    return React.createElement(View, { testID: "webview" });
  });
  return { WebView };
});

jest.mock("expo-notifications", () => ({
  AndroidImportance: { HIGH: 4 },
  useLastNotificationResponse: () => mockResponse,
  addPushTokenListener: () => ({ remove: () => undefined }),
  setNotificationHandler: jest.fn(),
  setNotificationChannelAsync: jest.fn(async () => null),
  setNotificationCategoryAsync: jest.fn(async () => null),
  getPermissionsAsync: jest.fn(async () => ({ granted: true })),
  requestPermissionsAsync: jest.fn(),
  getDevicePushTokenAsync: jest.fn(async () => ({ type: "android", data: "fcm-1" })),
  dismissNotificationAsync: jest.fn(),
  scheduleNotificationAsync: jest.fn(),
}));

const origin = "http://192.168.0.10:8783";
let fetchMock: jest.Mock;

beforeEach(async () => {
  await AsyncStorage.clear();
  mockWeb = null;
  mockResponse = null;
  jest.clearAllMocks();
  fetchMock = jest.fn(async () => new Response(null, { status: 204 }));
  globalThis.fetch = fetchMock as unknown as typeof fetch;
});

async function opened() {
  await AsyncStorage.setItem("switchboard.web-origin", origin);
  await render(<App />);
  await screen.findByTestId("webview");
}

describe("the first start", () => {
  it("asks for the WUI's URL and refuses one that is not http or https", async () => {
    await render(<App />);
    const input = await screen.findByLabelText("Web表示のURL");
    await fireEvent.changeText(input, "192.168.0.10");
    await fireEvent.press(screen.getByRole("button", { name: "保存する" }));
    expect(screen.getByRole("alert")).toHaveTextContent(/http か https/);
    expect(screen.queryByTestId("webview")).toBeNull();
  });

  it("opens the WUI once a URL is saved, and keeps it", async () => {
    await render(<App />);
    await fireEvent.changeText(await screen.findByLabelText("Web表示のURL"), "http://192.168.0.10:8783/");
    await fireEvent.press(screen.getByRole("button", { name: "保存する" }));
    await screen.findByTestId("webview");
    expect(mockWeb?.source.uri).toBe(`${origin}/`);
    await waitFor(async () => expect(await AsyncStorage.getItem("switchboard.web-origin")).toBe(origin));
  });
});

describe("the WUI", () => {
  it("registers the app's token with the WUI it shows", async () => {
    await opened();
    // The WebView also asks the WUI's /api/auth for the GitLab of the login; the registration is a call of its own.
    await waitFor(() => expect(fetchMock.mock.calls.some(([u]: [string]) => u === `${origin}/api/devices`)).toBe(true));
    const [url, init] = fetchMock.mock.calls.find(([u]: [string]) => u === `${origin}/api/devices`);
    expect(url).toBe(`${origin}/api/devices`);
    expect(init.headers.Origin).toBe(origin);
  });

  it("keeps the WUI's pages and opens other links in the default browser", async () => {
    const openURL = jest.spyOn(Linking, "openURL").mockResolvedValue(undefined);
    await opened();
    expect(mockWeb?.onShouldStartLoadWithRequest({ url: `${origin}/issue/gh%3Ao%2Fr%2312` })).toBe(true);
    expect(mockWeb?.onShouldStartLoadWithRequest({ url: "https://github.com/o/r/issues/12" })).toBe(false);
    expect(openURL).toHaveBeenCalledWith("https://github.com/o/r/issues/12");
  });

  it("goes back in the WebView while it can, and leaves back to Android after that", async () => {
    let onBack: (() => boolean) | undefined;
    jest.spyOn(BackHandler, "addEventListener").mockImplementation((_e, h) => {
      onBack = h as () => boolean;
      return { remove: () => undefined };
    });
    await opened();
    await act(() => mockWeb?.onNavigationStateChange({ canGoBack: true }));
    expect(onBack?.()).toBe(true);
    expect(mockGoBack).toHaveBeenCalled();
    await act(() => mockWeb?.onNavigationStateChange({ canGoBack: false }));
    expect(onBack?.()).toBe(false);
  });

  it("says so when the WUI cannot be reached, and leads to the settings", async () => {
    await opened();
    await act(() => mockWeb?.onError({ nativeEvent: { description: "net::ERR_CONNECTION_REFUSED" } }));
    expect(screen.getByRole("alert")).toHaveTextContent("受信箱のWeb表示に届きません");
    expect(screen.getByText("net::ERR_CONNECTION_REFUSED")).toBeTruthy();
    await fireEvent.press(screen.getByRole("button", { name: "接続先を変える" }));
    expect(screen.getByLabelText("Web表示のURL").props.value).toBe(origin);
    await fireEvent.press(screen.getByRole("button", { name: "やめる" }));
    await screen.findByTestId("webview");
  });

  it("changes the URL from the bar's button", async () => {
    await opened();
    await fireEvent.press(screen.getByRole("button", { name: "接続先" }));
    await fireEvent.changeText(screen.getByLabelText("Web表示のURL"), "http://192.168.0.20:8783");
    await fireEvent.press(screen.getByRole("button", { name: "保存する" }));
    await screen.findByTestId("webview");
    expect(mockWeb?.source.uri).toBe("http://192.168.0.20:8783/");
  });
});

describe("a pressed notification", () => {
  it("opens its issue in the WUI", async () => {
    mockResponse = {
      actionIdentifier: "expo.modules.notifications.actions.DEFAULT",
      notification: { request: { identifier: "app-1", content: { data: { ref: "gh:o/r#12", state: "waiting", reason: "question", since: "2026-11-02T09:30:00Z" } } } },
    };
    await opened();
    await waitFor(() => expect(mockWeb?.source.uri).toBe(`${origin}/issue/gh%3Ao%2Fr%2312`));
  });
});
