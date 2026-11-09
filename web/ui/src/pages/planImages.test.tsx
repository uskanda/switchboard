import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { projectBase } from "@/components/QuestionImage";

/** FakeEventSource stands in for the browser's EventSource; nothing is sent through it here. */
class FakeEventSource {
  onerror: (() => void) | null = null;
  addEventListener() {}
  close() {}
}

const t0 = "2026-11-09T10:00:00Z";
const ref = "gl:nas/sb#3";
const url = "https://gitlab.test/nas/sb/-/issues/3";
const item = { n: 1, state: "waiting", ref, project: "nas/sb", number: 3, title: "Layout", url,
  host: "laptop", waiting_since: t0, reason: "plan-questions", question: "Q1", updated_at: t0, notes: 0 };
const asked = (body: string) => ({ author: "agent", created_at: t0, body, agent: true, kind: "waiting" });
const person = (body: string) => ({ author: "me", created_at: t0, body, agent: false });

const qImage = "/uploads/0123456789abcdef0123456789abcdef/q.png";
const bImage = "/uploads/fedcba9876543210fedcba9876543210/b.png";
// As internal/agent/planrun.go writes a question with pictures on GitLab.
const withImages = (n: number) =>
  `### 計画についての質問\n\n**計画：** 保存のボタンを移す\n\n❓ **Q${n}** - **配置**\n\n保存のボタンをどこに置きますか。\n\n![q.png](${qImage})\n\n` +
  `A: 今のまま\nB: 上に移す\n![b.png](${bImage})\n\n➡️ **推奨：B** 最初に目に入る。\n`;

function serve(comments: unknown[]) {
  const last = comments.at(-1) as { body: string };
  const detail = { ref, title: "Layout", url, waiting: true, host: "laptop", state: "waiting", reason: "plan-questions",
    since: t0, post: last.body, approve_word: "/ok", author: "me", created_at: t0, body: "Move the button.", comments };
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    if (input === "/api/inbox") return new Response(JSON.stringify({ items: [item], errors: [], fetched_at: t0 }), { status: 200 });
    if (input.startsWith("/api/issue?")) return new Response(JSON.stringify(detail), { status: 200 });
    return new Response(JSON.stringify({ code: "not_found", message: input }), { status: 404 });
  }));
}

const open = async () => {
  render(
    <MemoryRouter initialEntries={["/issue/gl%3Anas%2Fsb%233"]}>
      <AppRoutes />
    </MemoryRouter>,
  );
  await screen.findByRole("button", { name: /この内容で返答|Send answers/ });
};

const through = (src: string) => `/api/issue-image?${new URLSearchParams({ issue: ref, src })}`;

beforeEach(() => vi.stubGlobal("EventSource", FakeEventSource));
afterEach(() => vi.unstubAllGlobals());

describe("the pictures of a plan question", () => {
  it("draws the question's picture before the options and an option's under it, read through this server", async () => {
    serve([asked(withImages(1))]);
    await open();
    const q = screen.getByRole("img", { name: "q.png" });
    expect(q.getAttribute("src")).toBe(through(qImage));
    const options = screen.getByRole("group", { name: "Q1の選択肢" });
    expect(q.compareDocumentPosition(options) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const b = within(options).getByRole("img", { name: "b.png" });
    expect(b.getAttribute("src")).toBe(through(bImage));
    expect(screen.queryByText(/画像：/)).toBeNull();
  });

  it("opens a picture at its own size", async () => {
    serve([asked(withImages(1))]);
    await open();
    HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) {
      this.open = true;
    };
    await userEvent.click(screen.getByRole("button", { name: "q.pngを大きく見る" }));
    const dialog = screen.getByRole("dialog", { name: "q.pngの画像" });
    expect(within(dialog).getByRole("img", { name: "q.png" }).getAttribute("src")).toBe(through(qImage));
  });

  it("falls back to a link to the forge when the picture cannot be read", async () => {
    serve([asked(withImages(1))]);
    await open();
    fireEvent.error(screen.getByRole("img", { name: "q.png" }));
    const link = screen.getByRole("link", { name: "画像：q.png" });
    expect(link.getAttribute("href")).toBe(`https://gitlab.test/nas/sb${qImage}`);
  });

  it("leaves the pictures of an earlier round as links", async () => {
    serve([asked(withImages(1)), person("Q1 配置: B"), asked(withImages(2))]);
    await open();
    expect(screen.getAllByRole("img", { name: "q.png" })).toHaveLength(1);
    expect(screen.getAllByRole("link", { name: /画像：q\.png/ }).length).toBeGreaterThan(0);
  });
});

describe("projectBase", () => {
  it("cuts the issue off the address of a GitLab or GitHub issue", () => {
    expect(projectBase("https://gitlab.test/g/sub/p/-/issues/3")).toBe("https://gitlab.test/g/sub/p");
    expect(projectBase("https://github.com/o/r/issues/9")).toBe("https://github.com/o/r");
    expect(projectBase("https://gl.uskan.dev/nas/scripts/-/work_items/23")).toBe("https://gl.uskan.dev/nas/scripts");
  });
});
