import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { GROUP_BY_PROJECT_KEY, readGroupByProject, useGroupByProject } from "@/lib/groupPref";

afterEach(() => window.localStorage.clear());

describe("readGroupByProject", () => {
  it("is on when nothing is stored", () => {
    expect(readGroupByProject(null)).toBe(true);
  });

  it("is off only for false", () => {
    expect(readGroupByProject("false")).toBe(false);
  });

  it("is on for a value it cannot read", () => {
    expect(readGroupByProject("maybe")).toBe(true);
    expect(readGroupByProject("true")).toBe(true);
  });
});

describe("useGroupByProject", () => {
  it("starts on in a browser that never chose", () => {
    const { result } = renderHook(() => useGroupByProject());
    expect(result.current[0]).toBe(true);
  });

  it("stores what is chosen, and starts with it next time", () => {
    const first = renderHook(() => useGroupByProject());
    act(() => first.result.current[1](false));
    expect(first.result.current[0]).toBe(false);
    expect(window.localStorage.getItem(GROUP_BY_PROJECT_KEY)).toBe("false");
    first.unmount();
    const next = renderHook(() => useGroupByProject());
    expect(next.result.current[0]).toBe(false);
  });
});
