import { describe, expect, it } from "vitest";
import { cn } from "@/lib/utils";

describe("cn", () => {
  it("lets a later spacing token or number replace an earlier one", () => {
    expect(cn("p-lg", "p-0")).toBe("p-0");
    expect(cn("gap-lg", "gap-sm")).toBe("gap-sm");
  });

  it("keeps a text size token apart from a text color", () => {
    expect(cn("text-label-md", "text-muted")).toBe("text-label-md text-muted");
    expect(cn("text-label-md", "text-headline-md")).toBe("text-headline-md");
  });

  it("knows the radius tokens", () => {
    expect(cn("rounded-md", "rounded-full")).toBe("rounded-full");
  });
});
