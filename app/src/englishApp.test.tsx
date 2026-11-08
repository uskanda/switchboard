import AsyncStorage from "@react-native-async-storage/async-storage";
import { act, fireEvent, render, screen } from "@testing-library/react-native";
import { App } from "./App";
import { deviceLang } from "./lib/i18n";

type WebProps = {
  source: unknown;
  onError: (e: { nativeEvent: { description?: string } }) => void;
};

let mockWeb: WebProps | null = null;

jest.mock("react-native-webview", () => {
  const React = require("react");
  const { View } = require("react-native");
  const WebView = React.forwardRef((props: WebProps, ref: unknown) => {
    React.useImperativeHandle(ref, () => ({ goBack: () => undefined }));
    mockWeb = props;
    return React.createElement(View, { testID: "webview" });
  });
  return { WebView };
});

jest.mock("expo-notifications", () => ({
  AndroidImportance: { HIGH: 4 },
  useLastNotificationResponse: () => null,
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

beforeEach(async () => {
  await AsyncStorage.clear();
  mockWeb = null;
  jest.clearAllMocks();
  globalThis.fetch = jest.fn(async () => new Response(null, { status: 204 })) as unknown as typeof fetch;
});

/** english makes the app run as on a phone set to English. */
function english() {
  jest.mocked(deviceLang).mockReturnValue("en");
}

async function opened() {
  await AsyncStorage.setItem("switchboard.web-origin", origin);
  await render(<App />);
  await screen.findByTestId("webview");
}

describe("the app on a phone set to English", () => {
  it("asks for the WUI's URL in English, and refuses one that is not http or https", async () => {
    english();
    await render(<App />);
    expect(await screen.findByRole("header")).toHaveTextContent("Web inbox address");
    expect(
      screen.getByText("Enter the IP address and port set as listen in web.yaml, starting with http://. The web inbox is reachable only from your home Wi-Fi."),
    ).toBeTruthy();
    await fireEvent.changeText(screen.getByLabelText("Web inbox URL"), "192.168.0.10");
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByRole("alert")).toHaveTextContent("Enter an absolute URL that starts with http or https (for example, http://192.168.0.10:8783).");
    expect(screen.queryByTestId("webview")).toBeNull();
  });

  it("names the bar's button and the settings' buttons in English", async () => {
    english();
    await opened();
    await fireEvent.press(screen.getByRole("button", { name: "Address" }));
    expect(screen.getByLabelText("Web inbox URL").props.value).toBe(origin);
    await fireEvent.press(screen.getByRole("button", { name: "Cancel" }));
    await screen.findByTestId("webview");
  });

  it("says in English that the WUI cannot be reached", async () => {
    english();
    await opened();
    await act(() => mockWeb?.onError({ nativeEvent: { description: "net::ERR_CONNECTION_REFUSED" } }));
    expect(screen.getByRole("alert")).toHaveTextContent("Cannot reach the web inbox");
    expect(screen.getByText(`Could not reach ${origin}. Check that you are on your home Wi-Fi and that the address is right.`)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Change address" })).toBeTruthy();
    await fireEvent.press(screen.getByRole("button", { name: "Reload" }));
    await screen.findByTestId("webview");
  });

  it("gives the WUI no language, so the WUI picks its own from the WebView", async () => {
    english();
    await opened();
    expect(mockWeb?.source).toEqual({ uri: `${origin}/` });
  });
});
