import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

type Answer = { status: number; body?: unknown };

/** answer makes fetch reply by "METHOD path" and records the requests. */
function answer(routes: Record<string, Answer | Answer[]>) {
  const calls: { key: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, init });
      const r = routes[key];
      // A list of answers is used in turn, the last one again and again.
      const a = Array.isArray(r) ? (r.length > 1 ? r.shift() : r[0]) : r;
      if (!a) return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
      return new Response(a.status === 204 ? null : JSON.stringify(a.body), { status: a.status });
    }),
  );
  return calls;
}

/** FakeEventSource stands in for the browser's EventSource; the test sends the events. */
class FakeEventSource {
  static last: FakeEventSource | null = null;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;
  private listeners = new Map<string, ((e: MessageEvent<string>) => void)[]>();
  constructor(readonly url: string) {
    FakeEventSource.last = this;
  }
  addEventListener(name: string, fn: (e: MessageEvent<string>) => void) {
    this.listeners.set(name, [...(this.listeners.get(name) ?? []), fn]);
  }
  close() {
    this.closed = true;
  }
  emit(name: string, body: unknown) {
    act(() => {
      this.onopen?.();
      for (const fn of this.listeners.get(name) ?? []) fn(new MessageEvent(name, { data: JSON.stringify(body) }));
    });
  }
  lose() {
    act(() => this.onerror?.());
  }
}

const events = () => {
  if (!FakeEventSource.last) throw new Error("no stream was opened");
  return FakeEventSource.last;
};

beforeEach(() => {
  FakeEventSource.last = null;
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => vi.unstubAllGlobals());

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  );
}

const item = {
  n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "<script>alert(1)</script>",
  url: "https://github.test/o/r/issues/12", host: "laptop", waiting_since: new Date(Date.now() - 3 * 3600_000).toISOString(),
  reason: "question", question: "Which default should --dry-run have?",
};
const other = { ...item, n: 2, ref: "gl:nas/sb#6", title: "Fix the build", reason: "error" };
const inbox = (items: unknown[], errors: unknown[] = []) => ({ items, errors, fetched_at: "" });

const detail = {
  ref: "gh:o/r#12", title: "<b>Add a flag</b>", url: "https://github.test/o/r/issues/12", waiting: true,
  host: "laptop", state: "waiting", reason: "permission", since: null,
  post: "### The agent has a question\n\n**Denied**\n\n- Bash: `rm -rf /`\n\n**Question**\n\nWhich default?",
};
const issueKey = "GET /api/issue?ref=gh%3Ao%2Fr%2312";

const list = () => screen.getByRole("navigation", { name: "待ちのissue" });

describe("the list", () => {
  it("lists the waiting issues from the stream, with issue text as text", async () => {
    answer({});
    at("/");
    expect(events().url).toBe("/api/events");
    events().emit("inbox", inbox([item, other]));
    expect(await within(list()).findByText("<script>alert(1)</script>")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "受信箱" })).toBeInTheDocument();
    expect(document.querySelector("script")).toBeNull();
    expect(within(list()).getByText("gh:o/r#12").closest("a")).toHaveAttribute("href", "/issue/gh%3Ao%2Fr%2312");
    expect(within(list()).getByText("question")).toBeInTheDocument();
    expect(within(list()).getByText("error")).toBeInTheDocument();
    expect(within(list()).getAllByText("3時間前")).toHaveLength(2);
    expect(within(list()).getAllByText("Which default should --dry-run have?")).toHaveLength(2);
    expect(screen.getByText("issueを選ぶと、ここに質問の全文が出ます")).toBeInTheDocument();
  });

  it("follows each list the stream sends", async () => {
    answer({});
    at("/");
    events().emit("inbox", inbox([item]));
    expect(await within(list()).findByText("gh:o/r#12")).toBeInTheDocument();
    events().emit("inbox", inbox([item, other]));
    expect(within(list()).getByText("gl:nas/sb#6")).toBeInTheDocument();
    events().emit("inbox", inbox([]));
    expect(screen.queryByText("gh:o/r#12")).toBeNull();
    expect(screen.getByText("待ちのissueはありません")).toBeInTheDocument();
  });

  it("warns of a forge that could not be read", async () => {
    answer({});
    at("/");
    events().emit("inbox", inbox([item], [{ forge: "gitlab", message: "connection refused" }]));
    expect(await screen.findByRole("alert")).toHaveTextContent("gitlab");
    expect(within(list()).getByText("gh:o/r#12")).toBeInTheDocument();
  });

  it("says so when the inbox service is not reachable, and recovers", async () => {
    answer({});
    at("/");
    events().emit("failure", { code: "unavailable", message: "not reachable" });
    expect(await screen.findByRole("alert")).toHaveTextContent("受信箱のサービス");
    events().emit("inbox", inbox([item]));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows whether it is following", async () => {
    answer({});
    at("/");
    events().emit("inbox", inbox([]));
    expect(screen.getByRole("status", { name: "更新の状態" })).toHaveTextContent("自動更新中");
    events().lose();
    expect(screen.getByRole("status", { name: "更新の状態" })).toHaveTextContent("つなぎ直しています");
  });

  it("closes the stream when the page goes", () => {
    answer({});
    const { unmount } = at("/");
    unmount();
    expect(events().closed).toBe(true);
  });
});

describe("one issue", () => {
  it("shows the whole post beside the list, with the item marked", async () => {
    answer({ [issueKey]: { status: 200, body: detail } });
    at("/issue/gh%3Ao%2Fr%2312");
    events().emit("inbox", inbox([item, other]));
    expect(await screen.findByRole("heading", { level: 2, name: "<b>Add a flag</b>" })).toBeInTheDocument();
    expect(screen.getByText(/Bash: `rm -rf \/`/)).toBeInTheDocument();
    expect(screen.getByText(/Which default\?/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /フォージで開く/ })).toHaveAttribute("href", detail.url);
    expect(within(list()).getByText("gh:o/r#12").closest("a")).toHaveAttribute("aria-current", "page");
    expect(within(list()).getByText("gl:nas/sb#6").closest("a")).not.toHaveAttribute("aria-current");
  });

  it("switches to the issue picked in the list", async () => {
    answer({
      [issueKey]: { status: 200, body: detail },
      "GET /api/issue?ref=gl%3Anas%2Fsb%236": { status: 200, body: { ...detail, ref: "gl:nas/sb#6", title: "Fix the build" } },
    });
    at("/issue/gh%3Ao%2Fr%2312");
    events().emit("inbox", inbox([item, other]));
    await screen.findByRole("heading", { level: 2, name: "<b>Add a flag</b>" });
    await userEvent.click(within(list()).getByText("gl:nas/sb#6"));
    expect(await screen.findByRole("heading", { level: 2, name: "Fix the build" })).toBeInTheDocument();
  });

  it("reads the issue again when its item changes, and not otherwise", async () => {
    const calls = answer({
      [issueKey]: [
        { status: 200, body: detail },
        { status: 200, body: { ...detail, post: "A second question" } },
      ],
    });
    at("/issue/gh%3Ao%2Fr%2312");
    events().emit("inbox", inbox([item]));
    await screen.findByText(/Which default\?/);
    events().emit("inbox", inbox([item, other]));
    const reads = () => calls.filter((c) => c.key === issueKey).length;
    expect(reads()).toBe(1);
    events().emit("inbox", inbox([{ ...item, waiting_since: new Date().toISOString(), question: "A second question" }]));
    expect(await screen.findByText("A second question", { selector: "p" })).toBeInTheDocument();
    expect(reads()).toBe(2);
  });

  it("reads the issue again when it leaves the list", async () => {
    answer({
      [issueKey]: [
        { status: 200, body: detail },
        { status: 200, body: { ...detail, waiting: false } },
      ],
    });
    at("/issue/gh%3Ao%2Fr%2312");
    events().emit("inbox", inbox([item]));
    await screen.findByRole("textbox", { name: "返答" });
    events().emit("inbox", inbox([]));
    expect(await screen.findByText(/待ちではありません/)).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "返答" })).toBeNull();
  });

  it("posts the reply and says it was sent", async () => {
    const calls = answer({ [issueKey]: { status: 200, body: detail }, "POST /api/reply": { status: 204 } });
    at("/issue/gh%3Ao%2Fr%2312");
    events().emit("inbox", inbox([item]));
    const send = await screen.findByRole("button", { name: "返答を送る" });
    expect(send).toBeDisabled();
    await userEvent.type(screen.getByRole("textbox", { name: "返答" }), "Go with B.");
    await userEvent.click(send);
    expect(await screen.findByText(/送りました/)).toBeInTheDocument();
    const post = calls.find((c) => c.key === "POST /api/reply");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gh:o/r#12", text: "Go with B." });
    expect(new Headers(post?.init?.headers).get("Content-Type")).toBe("application/json");
  });

  it("says when the issue stopped waiting before the reply", async () => {
    answer({
      [issueKey]: { status: 200, body: detail },
      "POST /api/reply": { status: 409, body: { code: "not_waiting", message: "gh:o/r#12 is not waiting" } },
    });
    at("/issue/gh%3Ao%2Fr%2312");
    events().emit("inbox", inbox([item]));
    await userEvent.type(await screen.findByRole("textbox", { name: "返答" }), "Go with B.");
    await userEvent.click(screen.getByRole("button", { name: "返答を送る" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("待ちではなくなりました"));
  });

  it("offers no reply on an issue that is not waiting", async () => {
    answer({ [issueKey]: { status: 200, body: { ...detail, waiting: false } } });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/待ちではありません/)).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "返答" })).toBeNull();
  });

  it("links back to the list for a narrow screen", async () => {
    answer({ [issueKey]: { status: 200, body: detail } });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByRole("link", { name: "← 一覧" })).toHaveAttribute("href", "/");
  });
});
