import { describe, expect, it } from "vitest";
import { diagramAllowed, withFocus } from "@/lib/diagram";

describe("diagramAllowed", () => {
  it("takes the three kinds of diagram", () => {
    expect(diagramAllowed("flowchart TD\n  A --> B")).toBe(true);
    expect(diagramAllowed("\nstateDiagram-v2\n  [*] --> A")).toBe(true);
    expect(diagramAllowed("sequenceDiagram\n  A->>B: hi")).toBe(true);
    expect(diagramAllowed("flowchart TD\n  A[style of the page] --> B:::focus")).toBe(true);
  });

  it("refuses another kind, a line that restyles and a directive", () => {
    for (const src of [
      "pie title x",
      "graph TD\n  A --> B",
      "flowchart TD\n  classDef warn fill:#f00",
      "flowchart TD\n  A --> B\n  style A fill:#f00",
      "flowchart TD\n  linkStyle 0 stroke:#f00",
      "flowchart TD\n  click A cb",
      "%%{init: {'theme': 'dark'}}%%\nflowchart TD\n  A --> B",
      "  ",
    ]) {
      expect(diagramAllowed(src), src).toBe(false);
    }
  });
});

describe("withFocus", () => {
  it("defines the focus class right after the first line of a flowchart or a state diagram", () => {
    expect(withFocus("flowchart TD\n  A --> B:::focus", "#1c1d1f")).toBe("flowchart TD\n  classDef focus stroke-width:3px,stroke:#1c1d1f\n  A --> B:::focus");
    expect(withFocus("stateDiagram-v2\n  A --> B", "#e8e6e1")).toBe("stateDiagram-v2\n  classDef focus stroke-width:3px,stroke:#e8e6e1\n  A --> B");
  });

  it("leaves a sequence diagram as it is", () => {
    expect(withFocus("sequenceDiagram\n  A->>B: hi", "#000")).toBe("sequenceDiagram\n  A->>B: hi");
  });
});
