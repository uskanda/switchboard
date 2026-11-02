import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

type Answer = { status: number; body?: unknown };

function answer(routes: Record<string, Answer | Answer[]>) {
  const calls: { key: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, init });
      const r = routes[key];
      const a = Array.isArray(r) ? (r.length > 1 ? r.shift() : r[0]) : r;
      if (!a && key === "GET /api/instructions") return new Response(JSON.stringify({ host: "ubuntu-a", items: [{ target: "", content: "" }] }), { status: 200 });
      if (!a) return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
      return new Response(a.status === 204 ? null : JSON.stringify(a.body), { status: a.status });
    }),
  );
  return calls;
}

afterEach(() => vi.unstubAllGlobals());

function settings() {
  return render(
    <MemoryRouter initialEntries={["/settings"]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const sandbox = { ref: "gh:o/r", path: "/home/me/repos/r", plan_first: false, cloned: true, missing_labels: [] };
const notes = { ref: "gh:o/notes", path: "/home/me/repos/notes", plan_first: true, cloned: false, missing_labels: ["agent:ready", "agent:plan"] };
const projects = (enabled: boolean, ...items: unknown[]) => ({ status: 200, body: { host: "ubuntu-a", enabled, items } });

describe("the projects of the settings", () => {
  it("come before the instructions, with their state", async () => {
    answer({ "GET /api/projects": projects(true, sandbox, notes) });
    settings();
    const section = await screen.findByRole("region", { name: "プロジェクト" });
    const instructions = screen.getByRole("heading", { name: "計画の実行への追加の指示" });
    expect(section.compareDocumentPosition(instructions) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const rows = within(section).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("gh:o/r");
    expect(rows[0]).toHaveTextContent("クローン済み");
    expect(rows[0]).toHaveTextContent("状態ラベルはそろっています");
    expect(rows[1]).toHaveTextContent("計画から始める");
    expect(rows[1]).toHaveTextContent("agent:ready、agent:plan");
  });

  it("say to check the agent's token when a project has no clone", async () => {
    answer({ "GET /api/projects": projects(true, notes) });
    settings();
    const row = await screen.findByRole("listitem");
    expect(row).toHaveTextContent("クローンがありません");
    expect(row).toHaveTextContent("エージェントのトークン");
  });

  it("say that labels could not be read", async () => {
    answer({ "GET /api/projects": projects(true, { ...sandbox, missing_labels: [], labels_error: "connection refused" }) });
    settings();
    expect(await screen.findByRole("listitem")).toHaveTextContent("状態ラベルを読めませんでした");
  });

  it("offer no form but how to enable it when adding is disabled", async () => {
    answer({ "GET /api/projects": projects(false, sandbox) });
    settings();
    const section = await screen.findByRole("region", { name: "プロジェクト" });
    await within(section).findByText(/inbox\.project_root/);
    expect(within(section).queryByRole("textbox")).toBeNull();
    expect(within(section).queryByRole("button", { name: "追加する" })).toBeNull();
  });

  it("start with plan first on, and ask for the reference only", async () => {
    answer({ "GET /api/projects": projects(true, sandbox) });
    settings();
    const section = await screen.findByRole("region", { name: "プロジェクト" });
    expect(await within(section).findByRole("checkbox", { name: /計画から始める/ })).toBeChecked();
    expect(within(section).getAllByRole("textbox")).toHaveLength(1);
    expect(within(section).getByRole("textbox", { name: "追加するプロジェクト" })).toBeInTheDocument();
  });

  it("add a project, list it again and name the webhook command", async () => {
    const calls = answer({
      "GET /api/projects": [projects(true, sandbox), projects(true, sandbox, notes)],
      "POST /api/projects": { status: 201, body: { project: notes, webhook: "switchboard relay hook add gh:o/notes" } },
    });
    settings();
    const field = await screen.findByRole("textbox", { name: "追加するプロジェクト" });
    await userEvent.type(field, "gh:o/notes");
    await userEvent.click(screen.getByRole("button", { name: "追加する" }));
    const section = screen.getByRole("region", { name: "プロジェクト" });
    await waitFor(() => expect(within(section).getAllByRole("listitem")).toHaveLength(2));
    // The addition is told in a toast; the webhook command stays under the section to be copied.
    const toast = (await screen.findByText(/を追加しました/)).closest<HTMLElement>("[data-sonner-toast]");
    expect(toast).toHaveClass("toast-success");
    expect(toast).toHaveTextContent("gh:o/notes を追加しました");
    expect(within(section).queryByText(/を追加しました/)).toBeNull();
    expect(within(section).getByText("switchboard relay hook add gh:o/notes")).toBeInTheDocument();
    expect(within(section).getByText(/登録するまでは、ポーリングで扱います/)).toBeInTheDocument();
    const post = calls.find((c) => c.key === "POST /api/projects");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gh:o/notes", plan_first: true });
    expect(new Headers(post?.init?.headers).get("Content-Type")).toBe("application/json");
    expect(field).toHaveValue("");
  });

  it("show why an addition was refused, in the form", async () => {
    answer({
      "GET /api/projects": projects(true, sandbox),
      "POST /api/projects": { status: 409, body: { code: "project_exists", message: "exists" } },
    });
    settings();
    const field = await screen.findByRole("textbox", { name: "追加するプロジェクト" });
    await userEvent.type(field, "gh:o/r");
    await userEvent.click(screen.getByRole("button", { name: "追加する" }));
    const form = field.closest("form") as HTMLElement;
    expect(await within(form).findByRole("alert")).toHaveTextContent("既に設定にあります");
    expect(field).toHaveValue("gh:o/r");
  });

  it("send plan first off when it is turned off", async () => {
    const calls = answer({
      "GET /api/projects": projects(true, sandbox),
      "POST /api/projects": { status: 201, body: { project: { ...notes, plan_first: false } } },
    });
    settings();
    await userEvent.click(await screen.findByRole("checkbox", { name: /計画から始める/ }));
    await userEvent.type(screen.getByRole("textbox", { name: "追加するプロジェクト" }), "gh:o/notes");
    await userEvent.click(screen.getByRole("button", { name: "追加する" }));
    await waitFor(() => expect(calls.some((c) => c.key === "POST /api/projects")).toBe(true));
    const post = calls.find((c) => c.key === "POST /api/projects");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gh:o/notes", plan_first: false });
    // Without a relay there is no webhook command to leave under the list.
    await screen.findByText(/を追加しました/);
    expect(screen.queryByText(/webhook/)).toBeNull();
  });

  it("read the instructions again, so that the new project has its field", async () => {
    answer({
      "GET /api/projects": [projects(true, sandbox), projects(true, sandbox, notes)],
      "POST /api/projects": { status: 201, body: { project: notes } },
      "GET /api/instructions": [
        { status: 200, body: { host: "ubuntu-a", items: [{ target: "", content: "" }, { target: "gh:o/r", content: "" }] } },
        { status: 200, body: { host: "ubuntu-a", items: [{ target: "", content: "" }, { target: "gh:o/r", content: "" }, { target: "gh:o/notes", content: "" }] } },
      ],
    });
    settings();
    await userEvent.type(await screen.findByRole("textbox", { name: "追加するプロジェクト" }), "gh:o/notes");
    await userEvent.click(screen.getByRole("button", { name: "追加する" }));
    expect((await screen.findAllByRole("textbox", { name: "gh:o/notes" }))[0]).toHaveValue("");
  });
});
