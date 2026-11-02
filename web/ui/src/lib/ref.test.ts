import { describe, expect, it } from "vitest";
import { forgeName, refText, splitRef } from "@/lib/ref";

describe("splitRef", () => {
  it("takes the GitHub prefix off an issue", () => {
    expect(splitRef("gh:o/r#12")).toEqual({ forge: "github", path: "o/r#12" });
  });

  it("takes the GitLab prefix off an issue", () => {
    expect(splitRef("gl:nas/sb#7")).toEqual({ forge: "gitlab", path: "nas/sb#7" });
  });

  it("takes the prefix off a project", () => {
    expect(splitRef("gh:o/r")).toEqual({ forge: "github", path: "o/r" });
    expect(splitRef("gl:group/sub/project")).toEqual({ forge: "gitlab", path: "group/sub/project" });
  });

  it("leaves a string of another shape as it is, without a forge", () => {
    expect(splitRef("o/r#12")).toEqual({ path: "o/r#12" });
    expect(splitRef("bb:o/r")).toEqual({ path: "bb:o/r" });
    expect(splitRef("gh:")).toEqual({ path: "gh:" });
    expect(splitRef("")).toEqual({ path: "" });
  });
});

describe("refText", () => {
  it("puts the forge's name after the path", () => {
    expect(refText("gh:o/r")).toBe("o/r GitHub");
    expect(refText("gl:nas/sb#7")).toBe("nas/sb#7 GitLab");
  });

  it("gives a string of another shape as it is", () => {
    expect(refText("o/r")).toBe("o/r");
  });
});

describe("forgeName", () => {
  it("spells the forges as they do", () => {
    expect(forgeName.github).toBe("GitHub");
    expect(forgeName.gitlab).toBe("GitLab");
  });
});
