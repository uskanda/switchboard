import { describe, expect, it } from "vitest";
import type { Item } from "@/lib/api";
import { groupByProject, projectRef } from "@/lib/group";

const item = (ref: string): Item => ({
  ref, project: ref.slice(3).split("#")[0], number: Number(ref.split("#")[1]), title: ref, url: "", host: "laptop",
  waiting_since: null, reason: "question", question: "",
});

describe("projectRef", () => {
  it("takes the issue's number off a reference", () => {
    expect(projectRef("gh:o/r#12")).toBe("gh:o/r");
  });

  it("keeps a GitLab group's path whole", () => {
    expect(projectRef("gl:g/sub/r#3")).toBe("gl:g/sub/r");
  });

  it("gives back a reference without a number as it is", () => {
    expect(projectRef("gh:o/r")).toBe("gh:o/r");
  });
});

describe("groupByProject", () => {
  it("is empty for no items", () => {
    expect(groupByProject([])).toEqual([]);
  });

  it("orders the groups by where each project first comes, and keeps the order inside each", () => {
    const items = [item("gh:c/d#1"), item("gh:a/b#2"), item("gh:c/d#3"), item("gh:a/b#4")];
    const groups = groupByProject(items);
    expect(groups.map((g) => g.project)).toEqual(["gh:c/d", "gh:a/b"]);
    expect(groups[0].items.map((it) => it.ref)).toEqual(["gh:c/d#1", "gh:c/d#3"]);
    expect(groups[1].items.map((it) => it.ref)).toEqual(["gh:a/b#2", "gh:a/b#4"]);
  });

  it("keeps a GitHub and a GitLab project of the same name apart", () => {
    const groups = groupByProject([item("gh:o/r#1"), item("gl:o/r#2")]);
    expect(groups.map((g) => g.project)).toEqual(["gh:o/r", "gl:o/r"]);
  });
});
