import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { setLanguageChoice } from "@/lib/i18n";

// Mermaid stands in for the real one: it records that it was loaded, how it was set up and what it drew, and draws
// an SVG that names its source. A source with "broken" fails, as a diagram Mermaid cannot read does.
const mm = vi.hoisted(() => ({ loaded: 0, configs: [] as Record<string, unknown>[], sources: [] as string[] }));
vi.mock("mermaid", () => {
  mm.loaded++;
  return {
    default: {
      initialize: (c: Record<string, unknown>) => mm.configs.push(c),
      render: async (id: string, src: string) => {
        if (src.includes("broken")) throw new Error("Parse error");
        mm.sources.push(src);
        return { svg: `<svg data-testid="drawn" id="${id}"><text>${src.split("\n").length} lines</text></svg>` };
      },
    },
  };
});

/** FakeEventSource stands in for the browser's EventSource; nothing is sent through it here. */
class FakeEventSource {
  onerror: (() => void) | null = null;
  addEventListener() {}
  close() {}
}

/** scheme fakes the OS colour scheme: dark or not, and a change the test makes. */
function scheme(dark: boolean) {
  const listeners: ((e: { matches: boolean }) => void)[] = [];
  const mq = { matches: dark, addEventListener: (_: string, fn: (e: { matches: boolean }) => void) => listeners.push(fn), removeEventListener: () => {} };
  vi.stubGlobal("matchMedia", (q: string) => (q.includes("dark") ? mq : { matches: false, addEventListener: () => {}, removeEventListener: () => {} }));
  return (next: boolean) => act(() => {
    mq.matches = next;
    listeners.forEach((fn) => fn({ matches: next }));
  });
}

const t0 = "2026-11-08T10:00:00Z";
const item = { n: 1, state: "waiting", ref: "gh:o/r#90", project: "o/r", number: 90, title: "Flow", url: "https://github.test/o/r/issues/90",
  host: "laptop", waiting_since: t0, reason: "plan-questions", question: "Q1", updated_at: t0, notes: 0 };
const asked = (body: string) => ({ author: "agent", created_at: t0, body, agent: true, kind: "waiting" });
const person = (body: string) => ({ author: "me", created_at: t0, body, agent: false });

/** questionWith is a question comment of one question with diagram (a mermaid block when given) before its options. */
const questionWith = (diagram?: string) =>
  `### 計画についての質問\n\n**計画：** 流れを分ける\n\n❓ **Q1** - **分かれ目**\n\nどこで分けますか。\n\n${diagram === undefined ? "" : "```mermaid\n" + diagram + "\n```\n\n"}A: 今のまま\nB: 分ける\n\n➡️ **推奨：B** 読み込みが速くなる\n`;

function serve(comments: unknown[]) {
  const last = comments.at(-1) as { body: string };
  const detail = { ref: "gh:o/r#90", title: "Flow", url: item.url, waiting: true, host: "laptop", state: "waiting", reason: "plan-questions",
    since: t0, post: last.body, approve_word: "/ok", author: "me", created_at: t0, body: "Split the flow.", comments };
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    if (input === "/api/inbox") return new Response(JSON.stringify({ items: [item], errors: [], fetched_at: t0 }), { status: 200 });
    if (input.startsWith("/api/issue?")) return new Response(JSON.stringify(detail), { status: 200 });
    return new Response(JSON.stringify({ code: "not_found", message: input }), { status: 404 });
  }));
}

const open = async () => {
  render(
    <MemoryRouter initialEntries={["/issue/gh%3Ao%2Fr%2390"]}>
      <AppRoutes />
    </MemoryRouter>,
  );
  await screen.findByRole("button", { name: /この内容で返答|Send answers/ });
};

beforeEach(() => {
  mm.loaded = 0;
  mm.configs.length = 0;
  mm.sources.length = 0;
  vi.stubGlobal("EventSource", FakeEventSource);
  scheme(false);
});
afterEach(() => vi.unstubAllGlobals());

/** follows reports whether b comes after a in the document. */
const follows = (a: Node, b: Node) => !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);

describe("the diagram of a question", () => {
  it("is drawn in the card between the body and the options, strictly, with dagre and the focus class", async () => {
    serve([asked(questionWith("flowchart TD\n  A[開く] --> B:::focus"))]);
    await open();
    const card = within(screen.getByRole("region", { name: "Q1 分かれ目" }));
    const drawn = await card.findByTestId("drawn");
    expect(follows(card.getByText("どこで分けますか。"), drawn)).toBe(true);
    expect(follows(drawn, screen.getByRole("group", { name: "Q1の選択肢" }))).toBe(true);
    expect(card.queryByText(/flowchart TD/)).toBeNull();
    expect(mm.configs.at(-1)).toMatchObject({ startOnLoad: false, securityLevel: "strict", layout: "dagre", theme: "neutral" });
    expect(mm.sources.at(-1)).toMatch(/^flowchart TD\n {2}classDef focus stroke-width:3px,stroke:[^\n]+\n {2}A\[開く\] --> B:::focus$/);
  });

  it("shows the recommendation under its option without the bold label", async () => {
    serve([asked(questionWith())]);
    await open();
    const card = within(screen.getByRole("region", { name: "Q1 分かれ目" }));
    expect(card.getByText("読み込みが速くなる")).toBeInTheDocument();
    expect(card.queryByText(/推奨：B/)).toBeNull();
  });

  it("does not load the drawing part for questions without a diagram", async () => {
    serve([asked(questionWith())]);
    await open();
    expect(mm.loaded).toBe(0);
  });

  it("opens at its own size in a dialog on a press, and closes", async () => {
    serve([asked(questionWith("flowchart TD\n  A --> B"))]);
    await open();
    await userEvent.click(await screen.findByRole("button", { name: "図を大きく見る" }));
    const dialog = screen.getByRole("dialog", { name: "Q1の図" });
    expect(within(dialog).getByTestId("drawn")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "閉じる" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("is drawn again in the dark theme when the OS turns dark", async () => {
    const turn = scheme(false);
    serve([asked(questionWith("flowchart TD\n  A --> B"))]);
    await open();
    await screen.findByTestId("drawn");
    turn(true);
    await waitFor(() => expect(mm.configs.at(-1)).toMatchObject({ theme: "dark" }));
  });

  it("is not drawn when it restyles, and its text is kept folded", async () => {
    serve([asked(questionWith("flowchart TD\n  A --> B\n  style A fill:#f00"))]);
    await open();
    const card = within(screen.getByRole("region", { name: "Q1 分かれ目" }));
    expect(card.getByText("図を描けませんでした")).toBeInTheDocument();
    const source = card.getByText(/style A fill/).closest("details")!;
    expect(source).not.toHaveAttribute("open");
    expect(mm.sources).toHaveLength(0);
  });

  it("says it could not be drawn when Mermaid cannot read it", async () => {
    serve([asked(questionWith("flowchart TD\n  A --> broken"))]);
    await open();
    expect(await screen.findByText("図を描けませんでした")).toBeInTheDocument();
    expect(screen.getByText(/A --> broken/).closest("details")).toBeInTheDocument();
  });

  it("is not drawn in an earlier round nor in a person's comment", async () => {
    const earlier = questionWith("flowchart TD\n  X --> Y");
    const later = questionWith().replace("**Q1** - **分かれ目**", "**Q2** - **次**");
    serve([asked(earlier), person("```mermaid\nflowchart TD\n  P --> Q\n```"), { ...asked(later), created_at: "2026-11-08T11:00:00Z" }]);
    await open();
    expect(screen.queryByTestId("drawn")).toBeNull();
    expect(screen.getByText(/P --> Q/).closest("code")).toBeInTheDocument();
    expect(mm.loaded).toBe(0);
  });

  it("speaks English on an English screen", async () => {
    setLanguageChoice("en");
    serve([asked(questionWith("flowchart TD\n  A --> B\n  classDef x fill:#f00"))]);
    await open();
    expect(screen.getByText("Couldn't draw the diagram")).toBeInTheDocument();
    expect(screen.getByText("The diagram's text")).toBeInTheDocument();
  });
});
