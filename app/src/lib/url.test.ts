import { isInside, issuePath, parseWebURL, staysInside } from "./url";

describe("parseWebURL", () => {
  it("keeps the origin of an http or https URL", () => {
    expect(parseWebURL(" http://192.168.0.10:8783 ")).toEqual({ ok: true, origin: "http://192.168.0.10:8783" });
    expect(parseWebURL("https://inbox.lan/issue/x?y")).toEqual({ ok: true, origin: "https://inbox.lan" });
    expect(parseWebURL("HTTP://192.168.0.10:8783/")).toEqual({ ok: true, origin: "http://192.168.0.10:8783" });
  });

  it.each(["192.168.0.10", "", "ftp://192.168.0.10/", "http://", "http://user:pw@192.168.0.10:8783", "javascript:alert(1)"])(
    "refuses %j",
    (input) => {
      const got = parseWebURL(input);
      expect(got.ok).toBe(false);
      if (!got.ok) expect(got.error).toContain("http");
    },
  );
});

describe("isInside", () => {
  const origin = "http://192.168.0.10:8783";
  it("tells the WUI's own pages from the rest", () => {
    expect(isInside(origin, "http://192.168.0.10:8783")).toBe(true);
    expect(isInside(origin, "http://192.168.0.10:8783/issue/gh%3Ao%2Fr%2312")).toBe(true);
    expect(isInside(origin, "http://192.168.0.10:8783?x")).toBe(true);
    expect(isInside(origin, "https://github.com/o/r/issues/12")).toBe(false);
    expect(isInside(origin, "http://192.168.0.10:87830/")).toBe(false);
    expect(isInside(origin, "http://192.168.0.10:8783.evil.example/")).toBe(false);
  });
});

describe("issuePath", () => {
  it("names one issue's page, with the merge dialog when asked", () => {
    expect(issuePath("gh:o/r#12")).toBe("/issue/gh%3Ao%2Fr%2312");
    expect(issuePath("gl:g/p#3", true)).toBe("/issue/gl%3Ag%2Fp%233?action=merge");
  });
});

describe("staysInside", () => {
  const origin = "http://192.168.0.10:8783";
  it("keeps the WUI and the GitLab of the login in the WebView", () => {
    expect(staysInside(origin, "https://gl.example", "http://192.168.0.10:8783/issue/x")).toBe(true);
    expect(staysInside(origin, "https://gl.example", "https://gl.example/users/sign_in")).toBe(true);
    expect(staysInside(origin, "https://gl.example/", "https://gl.example/oauth/authorize?x=1")).toBe(true);
    expect(staysInside(origin, "https://gl.example", "https://gl.example.evil.test/")).toBe(false);
    expect(staysInside(origin, "https://gl.example", "https://github.com/o/r")).toBe(false);
  });

  it("sends GitLab out without the login", () => {
    expect(staysInside(origin, null, "https://gl.example/users/sign_in")).toBe(false);
  });
});
