import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { setLanguageChoice } from "@/lib/i18n";
import { scrolled } from "@/test-observers";

/** FakeEventSource stands in for the browser's EventSource; nothing is sent through it here. */
class FakeEventSource {
  onerror: (() => void) | null = null;
  addEventListener() {}
  close() {}
}

const t0 = "2026-11-08T10:00:00Z";
const item = { n: 1, state: "waiting", ref: "gh:o/r#91", project: "o/r", number: 91, title: "Weigh", url: "https://github.test/o/r/issues/91",
  host: "laptop", waiting_since: t0, reason: "plan-questions", question: "Q5", updated_at: t0, notes: 0 };
const asked = (body: string, at = t0) => ({ author: "agent", created_at: at, body, agent: true, kind: "waiting" });
const person = (body: string) => ({ author: "me", created_at: t0, body, agent: false });

/** q is one question as switchboard writes it, with its options (labeled A, B…) and the recommended label. */
const q = (n: number, title: string, options = ["今のまま残す", "変える"], rec = "A") =>
  `❓ **Q${n}** - **${title}**\n\n${title}をどうしますか。\n\n${options.map((o, i) => `${String.fromCharCode(65 + i)}: ${o}`).join("\n")}\n\n➡️ **推奨：${rec}** 理由`;

/** round is a question comment: the questions that need a decision, then those under the heading of the checks. */
const round = (decisions: string[], checks: string[], heading = "#### 推奨どおりで進めるもの") =>
  "### 計画についての質問\n\n**計画：** 流れを分ける\n\n" +
  [...decisions, ...checks.map((c, i) => (i === 0 ? `${heading}\n\n${c}` : c))].join("\n\n---\n\n") +
  "\n\n推奨どおりでよければ `/ok`、変えたい問いがあれば `/ok q2はB` のように返答してください。\n";

const usual = round([q(5, "範囲"), q(6, "名前")], [q(7, "ログの形式"), q(8, "既定値", ["残す", "隠す", "消す"], "B"), q(9, "順番")]);

function serve(comments: unknown[]) {
  const last = comments.at(-1) as { body: string };
  const detail = { ref: "gh:o/r#91", title: "Weigh", url: item.url, waiting: true, host: "laptop", state: "waiting", reason: "plan-questions",
    since: t0, post: last.body, approve_word: "/ok", author: "me", created_at: t0, body: "Weigh the questions.", comments };
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    if (input === "/api/inbox") return new Response(JSON.stringify({ items: [item], errors: [], fetched_at: t0 }), { status: 200 });
    if (input.startsWith("/api/issue?")) return new Response(JSON.stringify(detail), { status: 200 });
    return new Response(JSON.stringify({ code: "not_found", message: input }), { status: 404 });
  }));
}

const open = async () => {
  render(
    <MemoryRouter initialEntries={["/issue/gh%3Ao%2Fr%2391"]}>
      <AppRoutes />
    </MemoryRouter>,
  );
  await screen.findByRole("button", { name: /この内容で返答|Send answers/ });
};

beforeEach(() => vi.stubGlobal("EventSource", FakeEventSource));
afterEach(() => vi.unstubAllGlobals());

const section = (name = "推奨どおりで進めるもの（3件）") => screen.getByRole("region", { name });
/** words is what a row says, without the arrow before it, which is hidden from screen readers. */
const words = (s: Element) => s.textContent!.replace(/^▸/, "");
const rows = (name?: string) => [...section(name).querySelectorAll("summary")].map(words);
const row = (n: number) => [...document.querySelectorAll("summary")].find((s) => words(s).startsWith(`Q${n} `)) as HTMLElement;
const chips = () => within(screen.getByRole("list", { name: "問いの状態" })).getAllByRole("button").map((b) => b.textContent);
const preview = () => screen.getByRole("region", { name: "「この内容で返答」で送る文" }).textContent;

describe("the questions that only confirm the recommended answer", () => {
  it("are a line each in a section after the cards, which count them in their place", async () => {
    serve([asked(usual)]);
    await open();
    expect(screen.getByRole("heading", { name: "Q5 範囲" }).textContent).toBe("Q5 1/5 範囲");
    expect(screen.getByRole("heading", { name: "Q6 名前" }).textContent).toBe("Q6 2/5 名前");
    expect(screen.queryByRole("region", { name: /^Q7 / })).toBeNull();
    expect(rows()).toEqual(["Q7 ログの形式 — A：今のまま残す", "Q8 既定値 — B：隠す", "Q9 順番 — A：今のまま残す"]);
    const follows = !!(screen.getByRole("region", { name: "Q6 名前" }).compareDocumentPosition(section()) & Node.DOCUMENT_POSITION_FOLLOWING);
    expect(follows).toBe(true);
  });

  it("open into their card on a press, without its title", async () => {
    serve([asked(usual)]);
    await open();
    const details = row(8).closest("details")!;
    expect(details).not.toHaveAttribute("open");
    await userEvent.click(row(8));
    expect(details).toHaveAttribute("open");
    expect(within(details).getByText("既定値をどうしますか。")).toBeVisible();
    expect(within(details).getByRole("group", { name: "Q8の選択肢" })).toBeInTheDocument();
    expect(within(details).queryByRole("heading")).toBeNull();
  });

  it("stay in their place with the answer and its state when chosen again, and keep it closed", async () => {
    serve([asked(usual)]);
    await open();
    await userEvent.click(row(8));
    await userEvent.click(within(screen.getByRole("group", { name: "Q8の選択肢" })).getByRole("radio", { name: /^C / }));
    expect(rows()).toEqual(["Q7 ログの形式 — A：今のまま残す", "Q8 既定値 — C：消す 推奨と違う", "Q9 順番 — A：今のまま残す"]);
    expect(row(8).closest("details")).toHaveAttribute("open");
    await userEvent.click(row(8));
    expect(row(8).closest("details")).not.toHaveAttribute("open");
    expect(preview()).toContain("Q8 既定値: C");
  });

  // A row is a <details> with its <summary>, so the browser opens it from the keyboard and tells whether it is open.
  // jsdom does neither; the check with Playwright covers them.

  it("are every question in the section when the comment holds checks alone", async () => {
    serve([asked(round([], [q(1, "ログの形式"), q(2, "順番")]))]);
    await open();
    expect(screen.queryByRole("heading", { name: /^Q1 / })).toBeNull();
    expect(screen.getByRole("region", { name: "推奨どおりで進めるもの（2件）" })).toBeInTheDocument();
    expect(chips()).toEqual(["推奨どおり 2件"]);
  });

  it("are all answered in the reply, a line each", async () => {
    serve([asked(usual)]);
    await open();
    expect(preview()).toBe("Q5 範囲: A\nQ6 名前: A\nQ7 ログの形式: A\nQ8 既定値: B\nQ9 順番: A");
  });

  it("are listed with what was settled, once answered", async () => {
    const next = round([q(10, "次")], []);
    serve([asked(usual, "2026-11-08T09:00:00Z"), person("Q5 範囲: A\nQ6 名前: A\nQ7 ログの形式: A\nQ8 既定値: C\nQ9 順番: A"), asked(next)]);
    await open();
    const settled = within(screen.getByRole("region", { name: "これまでに決まったこと" }));
    expect(settled.getByText("Q8 既定値")).toBeInTheDocument();
    expect(settled.getByText("Q9 順番")).toBeInTheDocument();
  });
});

describe("the chips with checks", () => {
  it("count the checks as recommended in one chip after the others", async () => {
    serve([asked(usual)]);
    await open();
    expect(chips()).toEqual(["Q5 推奨どおり", "Q6 推奨どおり", "推奨どおり 3件"]);
  });

  it("give a check chosen again a chip of its own", async () => {
    serve([asked(usual)]);
    await open();
    await userEvent.click(row(8));
    await userEvent.click(within(screen.getByRole("group", { name: "Q8の選択肢" })).getByRole("radio", { name: /^C / }));
    expect(chips()).toEqual(["Q5 推奨どおり", "Q6 推奨どおり", "Q8 推奨と違う", "推奨どおり 2件"]);
  });

  it("leave the chip of the checks out when none is as recommended", async () => {
    serve([asked(round([q(1, "範囲")], [q(2, "ログの形式")]))]);
    await open();
    await userEvent.click(row(2));
    await userEvent.click(within(screen.getByRole("group", { name: "Q2の選択肢" })).getByRole("radio", { name: /^B / }));
    expect(chips()).toEqual(["Q1 推奨どおり", "Q2 推奨と違う"]);
  });

  it("move to the section on a press of the chip of the checks", async () => {
    serve([asked(usual)]);
    await open();
    await userEvent.click(screen.getByRole("button", { name: "推奨どおり 3件" }));
    expect(screen.getByRole("heading", { name: "推奨どおりで進めるもの（3件）" })).toHaveFocus();
    expect(scrolled.at(-1)).toBe(section());
  });

  it("open the row of a check on a press of its chip", async () => {
    serve([asked(usual)]);
    await open();
    await userEvent.click(row(8));
    await userEvent.click(within(screen.getByRole("group", { name: "Q8の選択肢" })).getByRole("radio", { name: /^C / }));
    await userEvent.click(row(8));
    await userEvent.click(screen.getByRole("button", { name: "Q8 推奨と違う" }));
    expect(row(8).closest("details")).toHaveAttribute("open");
    expect(row(8)).toHaveFocus();
    expect(scrolled.at(-1)).toBe(row(8).closest("details"));
  });
});

describe("the checks on an English screen", () => {
  it("name the section and the chip in English", async () => {
    setLanguageChoice("en");
    serve([asked(round([q(1, "Scope")], [q(2, "Logs"), q(3, "Order")], "#### Going ahead as recommended"))]);
    await open();
    expect(screen.getByRole("region", { name: "Going ahead as recommended (2)" })).toBeInTheDocument();
    expect(within(screen.getByRole("list", { name: "Question status" })).getAllByRole("button").map((b) => b.textContent)).toEqual([
      "Q1 As recommended",
      "As recommended: 2",
    ]);
    expect(rows("Going ahead as recommended (2)")[0]).toBe("Q2 Logs — A: 今のまま残す");
    setLanguageChoice("auto");
  });
});
