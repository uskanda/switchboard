import { act, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

/** FakeEventSource stands in for the browser's EventSource; the test sends the stream's events through it. */
class FakeEventSource {
  static last: FakeEventSource | null = null;
  onerror: (() => void) | null = null;
  private listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  constructor(readonly url: string) {
    FakeEventSource.last = this;
  }
  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    (this.listeners[type] ??= []).push(fn);
  }
  close() {}
  emit(type: string, data: unknown) {
    act(() => this.listeners[type]?.forEach((fn) => fn(new MessageEvent(type, { data: JSON.stringify(data) }))));
  }
}

/**
 * serve answers GET /api/issue with what detail holds when the read is made; while hold is set, reads wait until
 * release is called.
 */
function serve(inbox: unknown) {
  const forge = {
    detail: null as unknown,
    hold: false,
    reads: 0,
    waiting: [] as (() => void)[],
    release() {
      const w = forge.waiting.splice(0);
      return act(async () => w.forEach((go) => go()));
    },
  };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string) => {
      if (input === "/api/inbox") return new Response(JSON.stringify(inbox), { status: 200 });
      if (input.startsWith("/api/issue?ref=")) {
        forge.reads++;
        const body = JSON.stringify(forge.detail);
        if (forge.hold) return new Promise<Response>((resolve) => forge.waiting.push(() => resolve(new Response(body, { status: 200 }))));
        return new Response(body, { status: 200 });
      }
      return new Response(JSON.stringify({ code: "not_found", message: input }), { status: 404 });
    }),
  );
  return forge;
}

beforeEach(() => {
  FakeEventSource.last = null;
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => vi.unstubAllGlobals());

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const t0 = "2026-10-31T10:00:00Z";
const item = (over: Record<string, unknown> = {}) => ({
  n: 1, state: "waiting", ref: "gh:o/r#72", project: "o/r", number: 72, title: "Add a flag", url: "https://github.test/o/r/issues/72",
  host: "laptop", waiting_since: t0, reason: "question", question: "Which default?", updated_at: t0, notes: 0, ...over,
});
const queuedRow = (over: Record<string, unknown> = {}) => item({ n: undefined, state: "queued", reason: "max-parallel", queued_since: t0, ...over });
const listing = (...items: unknown[]) => ({ items, errors: [], fetched_at: t0 });
const detail = (over: Record<string, unknown> = {}) => ({
  ref: "gh:o/r#72", title: "Add a flag", url: "https://github.test/o/r/issues/72", waiting: true, host: "laptop",
  state: "waiting", reason: "question", since: t0, post: "Which default?", author: "me", created_at: t0,
  body: "Please add --dry-run.", comments: [], ...over,
});
const running = () => detail({ waiting: false, state: "running", reason: "", post: "working" });

const NOTICE = "フォージから今読んだ状態です。一覧は更新を待っています";
const notice = () => screen.queryByRole("status", { name: NOTICE });

describe("a state the list has not caught up with", () => {
  it("is said to be read from the forge just now, while the list shows another", async () => {
    const forge = serve(listing(queuedRow()));
    forge.detail = running();
    at("/issue/gh%3Ao%2Fr%2372");
    FakeEventSource.last!.emit("inbox", listing(queuedRow()));
    await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    expect(await screen.findByRole("status", { name: NOTICE })).toHaveTextContent(NOTICE);
  });

  it("is not said when the list shows the same state, whatever the reason", async () => {
    const forge = serve(listing(item()));
    forge.detail = detail({ reason: "plan" });
    at("/issue/gh%3Ao%2Fr%2372");
    FakeEventSource.last!.emit("inbox", listing(item()));
    await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    await waitFor(() => expect(forge.reads).toBeGreaterThanOrEqual(1));
    await new Promise((r) => setTimeout(r, 20));
    expect(notice()).not.toBeInTheDocument();
  });

  it("is not said while the issue is read again", async () => {
    const forge = serve(listing(queuedRow()));
    forge.detail = running();
    at("/issue/gh%3Ao%2Fr%2372");
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(queuedRow()));
    await screen.findByRole("status", { name: NOTICE });

    forge.hold = true;
    es.emit("inbox", listing(queuedRow({ updated_at: "2026-10-31T10:05:00Z" })));
    expect(await screen.findByRole("status", { name: "更新を取得中" })).toBeInTheDocument();
    expect(notice()).not.toBeInTheDocument();
    await forge.release();
    expect(await screen.findByRole("status", { name: NOTICE })).toBeInTheDocument();
  });

  it("goes once the list catches up", async () => {
    const forge = serve(listing(queuedRow()));
    forge.detail = running();
    at("/issue/gh%3Ao%2Fr%2372");
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(queuedRow()));
    await screen.findByRole("status", { name: NOTICE });

    es.emit("inbox", listing(item({ n: undefined, state: "running", reason: "", running_since: t0, progress: "p", updated_at: "2026-10-31T10:05:00Z" })));
    await waitFor(() => expect(notice()).not.toBeInTheDocument());
    await waitFor(() => expect(screen.queryByRole("status", { name: "更新を取得中" })).not.toBeInTheDocument());
    expect(notice()).not.toBeInTheDocument();
  });
});
