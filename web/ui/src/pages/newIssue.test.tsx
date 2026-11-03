import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

type Answer = { status: number; body?: unknown };

/** answer makes fetch reply by "METHOD path" and records the requests. */
function answer(routes: Record<string, Answer>) {
  const calls: { key: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, init });
      const a = routes[key];
      if (!a && key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      if (!a) return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
      return new Response(JSON.stringify(a.body), { status: a.status });
    }),
  );
  return calls;
}

afterEach(() => vi.unstubAllGlobals());

const targets = {
  status: 200,
  body: {
    items: [
      { ref: "gh:o/r", host: "usknas" },
      { ref: "gl:nas/sb", host: "laptop" },
    ],
    starts: [
      { kind: "ready", label: "agent:ready" },
      { kind: "plan", label: "agent:plan" },
      { kind: "go", label: "agent:go" },
      { kind: "none", label: "" },
    ],
  },
};

async function openDialog() {
  render(
    <MemoryRouter initialEntries={["/"]}>
      <AppRoutes />
    </MemoryRouter>,
  );
  await userEvent.click(await screen.findByRole("button", { name: "新規issue" }));
  const dialog = await screen.findByRole("dialog", { name: "新規issue" });
  await within(dialog).findByRole("option", { name: "o/r GitHub（担当：usknas）" });
  return dialog;
}

describe("a new issue", () => {
  it("is written in a dialog over the list, starting ready in the first project", async () => {
    answer({ "GET /api/issue-targets": targets });
    const dialog = await openDialog();
    expect(within(dialog).getByRole("combobox", { name: "プロジェクト" })).toHaveValue("gh:o/r");
    expect(within(dialog).getByRole("option", { name: "nas/sb GitLab（担当：laptop）" })).toBeInTheDocument();
    const ready = within(dialog).getByRole("radio", { name: /agent:ready/ });
    expect(ready).toBeChecked();
    expect(within(dialog).getByRole("radio", { name: /agent:plan/ })).not.toBeChecked();
    expect(within(dialog).getByRole("radio", { name: /agent:go/ })).not.toBeChecked();
  });

  it("offers no label as the last start, without a label name", async () => {
    answer({ "GET /api/issue-targets": targets });
    const dialog = await openDialog();
    const radios = within(dialog).getAllByRole("radio");
    const none = radios[radios.length - 1];
    expect(none).toHaveAccessibleName("ラベルなし（実装開始しない）");
    expect(none).not.toBeChecked();
    expect(none.closest("label")?.querySelector(".font-mono")).toBeNull();
  });

  it("sends none as the kind of start when no label is chosen", async () => {
    const calls = answer({
      "GET /api/issue-targets": targets,
      "POST /api/issues": { status: 201, body: { ref: "gh:o/r#3", url: "https://github.test/o/r/issues/3" } },
    });
    const dialog = await openDialog();
    await userEvent.type(within(dialog).getByRole("textbox", { name: "タイトル" }), "Later");
    await userEvent.click(within(dialog).getByRole("radio", { name: "ラベルなし（実装開始しない）" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "作成する" }));

    expect(await screen.findByRole("status")).toHaveTextContent("o/r#3 GitHub を作りました");
    const post = calls.find((c) => c.key === "POST /api/issues");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gh:o/r", title: "Later", body: "", start: "none" });
  });

  it("cannot be sent without a title", async () => {
    answer({ "GET /api/issue-targets": targets });
    const dialog = await openDialog();
    const send = within(dialog).getByRole("button", { name: "作成する" });
    expect(send).toBeDisabled();
    await userEvent.type(within(dialog).getByRole("textbox", { name: "本文" }), "only a body");
    expect(send).toBeDisabled();
    await userEvent.type(within(dialog).getByRole("textbox", { name: "タイトル" }), "   ");
    expect(send).toBeDisabled();
  });

  it("sends the project, the title, the body and the kind of start, then closes and links to the forge", async () => {
    const calls = answer({
      "GET /api/issue-targets": targets,
      "POST /api/issues": { status: 201, body: { ref: "gl:nas/sb#7", url: "https://gitlab.test/nas/sb/-/issues/7" } },
    });
    const dialog = await openDialog();
    await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "プロジェクト" }), "gl:nas/sb");
    await userEvent.type(within(dialog).getByRole("textbox", { name: "タイトル" }), "Add a flag");
    await userEvent.type(within(dialog).getByRole("textbox", { name: "本文" }), "Use --x.");
    await userEvent.click(within(dialog).getByRole("radio", { name: /agent:plan/ }));
    await userEvent.click(within(dialog).getByRole("button", { name: "作成する" }));

    const status = await screen.findByRole("status");
    expect(status).toHaveTextContent("nas/sb#7 GitLab を作りました");
    expect(within(status).getByText("nas/sb#7").nextElementSibling).toHaveTextContent("GitLab");
    const toast = status.closest<HTMLElement>("[data-sonner-toast]")!;
    expect(toast).toHaveClass("toast-success");
    const link = within(toast).getByRole("link", { name: /フォージで開く/ });
    expect(link).toHaveAttribute("href", "https://gitlab.test/nas/sb/-/issues/7");
    expect(link).toHaveAttribute("target", "_blank");
    expect(screen.queryByRole("dialog")).toBeNull();

    const post = calls.find((c) => c.key === "POST /api/issues");
    expect((post?.init?.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gl:nas/sb", title: "Add a flag", body: "Use --x.", start: "plan" });

    // The notice is a toast, not a line above the list: the pane has nothing left of it.
    const pane = screen.getByRole("button", { name: "新規issue" }).parentElement!.parentElement!;
    expect(within(pane).queryByText(/を作りました/)).toBeNull();

    await userEvent.click(within(toast).getByRole("button", { name: "閉じる" }));
    await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  });

  it("keeps what was written and shows why it was refused", async () => {
    answer({
      "GET /api/issue-targets": targets,
      "POST /api/issues": { status: 502, body: { code: "forge", message: "rate limited" } },
    });
    const dialog = await openDialog();
    await userEvent.type(within(dialog).getByRole("textbox", { name: "タイトル" }), "Add a flag");
    await userEvent.type(within(dialog).getByRole("textbox", { name: "本文" }), "Use --x.");
    await userEvent.click(within(dialog).getByRole("button", { name: "作成する" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("rate limited");
    expect(screen.getByRole("dialog", { name: "新規issue" })).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: "タイトル" })).toHaveValue("Add a flag");
    expect(within(dialog).getByRole("textbox", { name: "本文" })).toHaveValue("Use --x.");
    await waitFor(() => expect(within(dialog).getByRole("button", { name: "作成する" })).toBeEnabled());
  });

  it("is closed without sending by the cancel button", async () => {
    const calls = answer({ "GET /api/issue-targets": targets });
    const dialog = await openDialog();
    await userEvent.click(within(dialog).getByRole("button", { name: "やめる" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(calls.some((c) => c.key === "POST /api/issues")).toBe(false);
  });
});
