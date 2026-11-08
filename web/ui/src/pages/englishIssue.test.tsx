import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { Markdown } from "@/components/Markdown";
import { Thread } from "@/components/Thread";
import type { Comment, Detail } from "@/lib/api";
import { setLanguageChoice } from "@/lib/i18n";

/** answer serves the routes given, an empty list and a 404 for anything else, and keeps every call. */
function answer(routes: Record<string, unknown>) {
  const calls: { key: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, init });
      if (key === "POST /api/reply" && key in routes) {
        const r = routes[key] as { status?: number; body?: unknown } | null;
        if (r?.status) return new Response(JSON.stringify(r.body), { status: r.status });
        return new Response(null, { status: 204 });
      }
      if (key in routes) return new Response(JSON.stringify(routes[key]), { status: 200 });
      if (key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
    }),
  );
  return calls;
}

afterEach(() => vi.unstubAllGlobals());

const at = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );

const issue = "GET /api/issue?ref=gh%3Ao%2Fr%2312";
const path = "/issue/gh%3Ao%2Fr%2312";
const t0 = "2026-10-31T12:00:00Z";
const asked = (body: string): Comment => ({ author: "bot", created_at: "2026-10-31T12:05:00Z", body, agent: true, kind: "waiting" });
const detail = (reason: string, body: string, over: Record<string, unknown> = {}) => ({
  ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true, host: "laptop",
  state: "waiting", reason, since: null, approve_word: "/ok", post: body, author: "me", created_at: t0,
  body: "Add a --dry-run flag.", comments: [asked(body)], ...over,
});
// The issue on the list, so that a reply sent to it is said to be sent.
const inboxOf = (reason: string) => ({
  items: [{ n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "u", host: "laptop", waiting_since: null, reason, question: "" }],
  errors: [],
  fetched_at: "",
});
const posted = (calls: { key: string; init?: RequestInit }[]) =>
  calls.filter((c) => c.key === "POST /api/reply").map((c) => JSON.parse(String(c.init?.body)) as { ref: string; text: string });

describe("the one-issue screen in English", () => {
  it("names the reply field and its button in English and shows the agent's question as it is", async () => {
    setLanguageChoice("en");
    const question = "### 質問があります\n\nフラグの名前は `--dry-run` でよいですか。";
    answer({ [issue]: detail("question", question) });
    at(path);
    const field = await screen.findByRole("textbox", { name: "Reply" });
    expect(field).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send reply" })).toBeDisabled();
    expect(screen.getByText(/フラグの名前は/)).toHaveTextContent("フラグの名前は --dry-run でよいですか。");
    expect(screen.getByRole("link", { name: "Open on the forge (new tab)" })).toBeInTheDocument();
    expect(screen.queryByText("返答を送る")).toBeNull();
  });

  it("says why a reply was not sent, with the server's own words after the lead", async () => {
    setLanguageChoice("en");
    answer({ [issue]: detail("question", "Which name?"), "POST /api/reply": { status: 500, body: { code: "internal", message: "サーバーの誤り" } } });
    at(path);
    await userEvent.type(await screen.findByRole("textbox", { name: "Reply" }), "dry-run");
    await userEvent.click(screen.getByRole("button", { name: "Send reply" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Failed: サーバーの誤り");
  });

  it("puts the server's English words after the Japanese lead on a Japanese screen", async () => {
    answer({ [issue]: detail("question", "Which name?"), "POST /api/reply": { status: 500, body: { code: "internal", message: "internal error" } } });
    at(path);
    await userEvent.type(await screen.findByRole("textbox", { name: "返答" }), "dry-run");
    await userEvent.click(screen.getByRole("button", { name: "返答を送る" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("失敗しました：internal error");
    expect(document.documentElement.lang).toBe("ja");
  });

  it("offers to approve a plan with the approval word", async () => {
    setLanguageChoice("en");
    const calls = answer({ "GET /api/inbox": inboxOf("plan"), [issue]: detail("plan", "### Plan\n\n1. Add the flag."), "POST /api/reply": null });
    at(path);
    expect(await screen.findByText(/To go ahead with this plan/)).toHaveTextContent("/ok");
    await userEvent.click(screen.getByRole("button", { name: "Approve (sends /ok)" }));
    expect(await within(screen.getByRole("article")).findByRole("status")).toHaveTextContent("Sent. The agent resumes the work.");
    expect(posted(calls)).toEqual([{ ref: "gh:o/r#12", text: "/ok" }]);
  });
});

describe("running again in English", () => {
  it("posts the English request to run again", async () => {
    setLanguageChoice("en");
    const calls = answer({ "GET /api/inbox": inboxOf("error"), [issue]: detail("error", "The run stopped."), "POST /api/reply": null });
    at(path);
    expect(await screen.findByText(/The agent's run failed/)).toHaveTextContent('"Run again"');
    await userEvent.click(screen.getByRole("button", { name: "Run again" }));
    await within(screen.getByRole("article")).findByRole("status");
    expect(posted(calls)).toEqual([{ ref: "gh:o/r#12", text: "Please run it again." }]);
  });

  it("posts the English request to resolve a conflict", async () => {
    setLanguageChoice("en");
    const calls = answer({ "GET /api/inbox": inboxOf("conflict"), [issue]: detail("conflict", "It still conflicts."), "POST /api/reply": null });
    at(path);
    expect(await screen.findByText(/could not be fixed automatically/)).toHaveTextContent("resets the count of automatic fixes");
    await userEvent.click(screen.getByRole("button", { name: "Run again" }));
    await within(screen.getByRole("article")).findByRole("status");
    expect(posted(calls)).toEqual([{ ref: "gh:o/r#12", text: "Resolve the conflict with the base branch and push again." }]);
  });
});

/** An English question comment of a plan run: two questions of one group, and one of two named groups. */
const questions = [
  "### The agent has questions for the plan",
  "",
  "❓ **Q1** - **Entry point**",
  "",
  "Where does the action go?",
  "A: Right-click only.",
  "B: The selection menu.",
  "",
  "➡️ A, as the issue asks.",
  "",
  "---",
  "",
  "❓ **Q2** - **Defaults**",
  "",
  "Scope: A all / B feed",
  "Field: X title and body / Y title only",
  "",
  "➡️ Scope A, field X.",
  "",
  "---",
  "",
  "❓ **Q3** - **Android**",
  "",
  "A: Web only.",
  "B: Both.",
  "",
  "➡️ A.",
  "",
  "Reply `/ok` to accept the recommended answers, or `/ok q2 is B` to change some of them.",
].join("\n");

describe("the questions of a plan in English", () => {
  it("are drawn in English, with the recommended option marked", async () => {
    setLanguageChoice("en");
    answer({ [issue]: detail("plan-questions", questions) });
    at(path);
    await screen.findByRole("button", { name: "Send answers" });
    const q1 = within(screen.getByRole("group", { name: "Q1 options" }));
    expect(q1.getByRole("radio", { name: /^A Right-click only\. \(recommended\)$/ })).toBeChecked();
    expect(q1.queryByRole("radio", { name: "None (answer in the note)" })).toBeNull();
    expect(screen.getByRole("group", { name: "Scope" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Write another answer or a note for Q1" })).toBeInTheDocument();
    // Only the question of two groups keeps the recommendation under its own label; the others have it under the option.
    expect(screen.getAllByText("Recommended")).toHaveLength(1);
    expect(screen.getByText("A, as the issue asks.")).toBeInTheDocument();
    expect(screen.getByText(/Choose an answer to each question/)).toHaveTextContent('"Send answers"');
    expect(screen.queryByText(/to accept the recommended answers/)).toBeNull();
  });

  it("post the answers in English", async () => {
    setLanguageChoice("en");
    const calls = answer({ "GET /api/inbox": inboxOf("plan-questions"), [issue]: detail("plan-questions", questions), "POST /api/reply": null });
    at(path);
    const send = await screen.findByRole("button", { name: "Send answers" });
    await userEvent.click(within(screen.getByRole("group", { name: "Q1 options" })).getByRole("radio", { name: /^B / }));
    await userEvent.click(screen.getByRole("button", { name: "Write another answer or a note for Q1" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Q1 note" }), "keep it small");
    await userEvent.click(screen.getByRole("button", { name: "Write another answer or a note for Q3" }));
    await userEvent.click(screen.getByRole("checkbox", { name: "Q3: answer in the note alone, without an option" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Reply" }), "no rush");
    const text = "Q1 Entry point: B. Note: keep it small\nQ2 Defaults: Scope: A; Field: X\nQ3 Android: No answer\n\nOverall note:\nno rush";
    expect(screen.getByRole("region", { name: 'What "Send answers" posts' }).textContent).toBe(text);
    await userEvent.click(send);
    expect(await within(screen.getByRole("article")).findByRole("status")).toHaveTextContent("Sent");
    expect(posted(calls)).toEqual([{ ref: "gh:o/r#12", text }]);
  });
});

describe("the cards and chips of a plan's questions in English", () => {
  it("say where every answer stands, in English", async () => {
    setLanguageChoice("en");
    answer({ [issue]: detail("plan-questions", questions) });
    at(path);
    await screen.findByRole("button", { name: "Send answers" });
    const chips = () => within(screen.getByRole("list", { name: "Question status" })).getAllByRole("button").map((b) => b.textContent);
    expect(chips()).toEqual(["Q1 As recommended", "Q2 As recommended", "Q3 As recommended"]);
    await userEvent.click(within(screen.getByRole("group", { name: "Q1 options" })).getByRole("radio", { name: /^B / }));
    await userEvent.click(screen.getByRole("button", { name: "Write another answer or a note for Q3" }));
    await userEvent.click(screen.getByRole("checkbox", { name: "Q3: answer in the note alone, without an option" }));
    expect(chips()).toEqual(["Q1 Changed", "Q2 As recommended", "Q3 Unanswered"]);
    expect(screen.getByText("Answer in the note alone, without an option")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Q1 Entry point" })).getByText("1/3")).toBeInTheDocument();
  });

  it("list what earlier rounds settled, in English", async () => {
    setLanguageChoice("en");
    const first = "### The agent has questions for the plan\n\n❓ **Q1** - **Before**\n\nA: x\nB: y\n\n➡️ A\n";
    const next = "### The agent has questions for the plan\n\n**Plan:** Filters\n\nI read a.go.\n\n❓ **Q2** - **Next**\n\nA: p\nB: q\n\n➡️ B\n";
    const comments = [agent(1, "waiting", first), person(2, "/ok"), agent(3, "running"), agent(4, "waiting", next)];
    answer({ [issue]: { ...detail("plan-questions", next), comments } });
    at(path);
    await screen.findByRole("button", { name: "Send answers" });
    const settled = within(screen.getByRole("region", { name: "Settled so far" }));
    expect(settled.getByText("Q1 Before")).toBeInTheDocument();
    expect(settled.getByText("A x")).toBeInTheDocument();
    expect(screen.getByText("What the agent looked at")).toBeInTheDocument();
  });
});

const agent = (m: number, kind: string, body = kind): Comment => ({ author: "bot", created_at: `2026-10-31T12:0${m}:00Z`, body, agent: true, kind });
const person = (m: number, body: string): Comment => ({ author: "me", created_at: `2026-10-31T12:0${m}:00Z`, body, agent: false });
const threadOf = (comments: Comment[], body = "本文"): Detail => ({
  ref: "gh:o/r#1", title: "t", url: "u", waiting: true, host: "h", state: "waiting", reason: "question", since: null, post: "", body, comments,
});

describe("the thread in English", () => {
  it("counts the replies of each answered round and marks the agent's posts", () => {
    setLanguageChoice("en");
    const comments = [
      agent(1, "waiting", "First question"), person(2, "One"), person(3, "Two"), agent(4, "running"),
      agent(5, "waiting", "Second question"), person(6, "Three"), agent(7, "running"),
      agent(8, "waiting", "Third question"), agent(9, "running"),
      agent(9, "waiting", "Latest question"),
    ];
    render(<Thread detail={threadOf(comments)} />);
    const summaries = screen.getAllByText(/^Answered question/).map((s) => s.textContent);
    expect(summaries).toEqual(["Answered question, 2 replies", "Answered question, 1 reply", "Answered question, no replies"]);
    expect(screen.getAllByText("Started")).toHaveLength(3);
    expect(screen.getAllByText("agent").length).toBeGreaterThan(0);
    expect(screen.queryByText("エージェント")).toBeNull();
    expect(screen.getByRole("list", { name: "Thread" })).toBeInTheDocument();
  });

  it("writes the time of a comment in English", () => {
    setLanguageChoice("en");
    const { container } = render(<Thread detail={threadOf([person(1, "Hi")])} />);
    const times = [...container.querySelectorAll("time")].map((t) => t.textContent);
    expect(times[1]).toMatch(/^Oct 31, 2026, \d{2}:\d{2}$/);
  });

  it("counts the lines of a progress log", () => {
    setLanguageChoice("en");
    const one = "### Progress of the run\n\n- 12:01 1m Read 2 | Reading the settings";
    const { unmount } = render(<Thread detail={threadOf([agent(1, "progress", one)])} />);
    expect(screen.getByText("Progress log (1 line)")).toBeInTheDocument();
    unmount();
    render(<Thread detail={threadOf([agent(1, "progress", `${one}\n- 12:02 2m Edit 1 |`)])} />);
    expect(screen.getByText("Progress log (2 lines)")).toBeInTheDocument();
    // Words not yet said read as working.
    expect(screen.getAllByText("Working").length).toBeGreaterThan(0);
  });

  it("says an issue without a body has none, and a reply just sent is not on the forge yet", () => {
    setLanguageChoice("en");
    render(<Thread detail={threadOf([], "")} pendingReply="On my way" />);
    expect(screen.getByText("No description")).toBeInTheDocument();
    expect(screen.getByText("You")).toBeInTheDocument();
    expect(screen.getByText("Sent, not yet on the forge")).toBeInTheDocument();
  });

  it("opens a round on pressing its line", () => {
    setLanguageChoice("en");
    render(<Thread detail={threadOf([agent(1, "waiting", "Q"), person(2, "A"), agent(3, "running"), agent(4, "waiting", "Next")])} />);
    fireEvent.click(screen.getByText("Answered question, 1 reply"));
    expect(screen.getByText("A")).toBeVisible();
  });
});

describe("issue text in English", () => {
  it("draws an image as a link named in English", () => {
    setLanguageChoice("en");
    render(<Markdown text="![shot](https://example.test/a.png)" />);
    expect(screen.getByRole("link", { name: "Image: shot" })).toHaveAttribute("href", "https://example.test/a.png");
  });
});
