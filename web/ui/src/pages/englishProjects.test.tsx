import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { setLanguageChoice } from "@/lib/i18n";

type Answer = { status: number; body?: unknown };

/** answer makes fetch reply by "METHOD path", in turn when given a list, and records the requests. */
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
      if (!a && key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      if (!a) return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
      return new Response(a.status === 204 ? null : JSON.stringify(a.body), { status: a.status });
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

const sandbox = { ref: "gh:o/r", path: "/home/me/repos/r", plan_first: false, plan_first_mode: "never", cloned: true, missing_labels: [] };
const notes = {
  ref: "gh:o/notes",
  path: "/home/me/repos/notes",
  plan_first: true,
  plan_first_mode: "auto",
  cloned: false,
  missing_labels: ["agent:ready", "agent:plan"],
};
const projects = (enabled: boolean, ...items: unknown[]) => ({ status: 200, body: { host: "ubuntu-a", enabled, items } });

describe("the projects of the settings in English", () => {
  it("name the section, the host, each project's start and what a project still needs", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/projects": projects(true, sandbox, notes) });
    at("/settings");
    const section = await screen.findByRole("region", { name: "Projects" });
    expect(within(section).getByRole("heading", { name: "Projects" })).toBeInTheDocument();
    await waitFor(() => expect(within(section).getAllByRole("listitem")).toHaveLength(2));
    expect(section).toHaveTextContent("The agent on host ubuntu-a works on these projects.");
    expect(within(section).getByText("ubuntu-a")).toHaveClass("font-mono");
    const rows = within(section).getAllByRole("listitem");
    expect(rows[0]).toHaveTextContent("/home/me/repos/notes · Start with a plan if needed");
    expect(rows[0]).toHaveTextContent("Not cloned yet.");
    expect(rows[0]).toHaveTextContent("check that the agent's token covers this repository");
    expect(rows[0]).toHaveTextContent("Missing state labels: agent:ready, agent:plan");
    expect(rows[1]).toHaveTextContent("o/r GitHub · Implement right away");
    expect(section).not.toHaveTextContent(/[ぁ-んァ-ヶ一-龠（）：、・]/);
  });

  it("say when the labels are all there, or could not be read", async () => {
    setLanguageChoice("en");
    const unread = { ...sandbox, ref: "gh:o/u", labels_error: "connection refused" };
    const uncloned = { ...sandbox, ref: "gh:o/c", cloned: false };
    const missing = { ...sandbox, ref: "gh:o/m", missing_labels: ["agent:go"] };
    answer({ "GET /api/projects": projects(true, unread, uncloned, missing) });
    at("/settings");
    const section = await screen.findByRole("region", { name: "Projects" });
    await waitFor(() => expect(within(section).getAllByRole("listitem")).toHaveLength(3));
    const rows = within(section).getAllByRole("listitem");
    expect(rows[0]).toHaveTextContent("Cloned");
    expect(rows[0]).toHaveTextContent("Could not read the state labels: connection refused");
    expect(rows[1]).toHaveTextContent("All state labels exist");
    expect(rows[2]).toHaveTextContent("Missing state labels: agent:go");
  });

  it("offer the form to add a project, with the three starts in English", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/projects": projects(true, sandbox) });
    at("/settings");
    const section = await screen.findByRole("region", { name: "Projects" });
    const field = await within(section).findByRole("textbox", { name: "Project to add" });
    expect(field).toHaveAttribute("placeholder", "gh:owner/repo");
    const group = within(section).getByRole("group", { name: "Starting with agent:ready" });
    const radios = within(group).getAllByRole("radio");
    expect(radios.map((r) => r.closest("label")?.textContent)).toEqual([
      "Start with a plan if needed (skip questions and approval when the requirements are clear)",
      "Always start with a plan (plan first and wait for approval)",
      "Implement right away (skip the plan)",
    ]);
    expect(radios[0]).toBeChecked();
    expect(within(section).getByRole("button", { name: "Add" })).toBeDisabled();
  });

  it("say how to enable adding when it is disabled", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/projects": projects(false, sandbox) });
    at("/settings");
    const section = await screen.findByRole("region", { name: "Projects" });
    const help = await within(section).findByText(/To add projects here/);
    expect(help).toHaveTextContent("To add projects here, set inbox.project_root (the parent directory of the clones) in config.yaml.");
    expect(within(help).getByText("inbox.project_root").tagName).toBe("CODE");
  });

  it("add a project and tell it in English, with the webhook command under the list", async () => {
    setLanguageChoice("en");
    answer({
      "GET /api/projects": [projects(true, sandbox), projects(true, sandbox, notes)],
      "POST /api/projects": { status: 201, body: { project: notes, webhook: "switchboard relay hook add gh:o/notes" } },
    });
    at("/settings");
    await userEvent.type(await screen.findByRole("textbox", { name: "Project to add" }), "gh:o/notes");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    const toast = (await screen.findByText(/^Added/)).closest<HTMLElement>("[data-sonner-toast]")!;
    expect(toast).toHaveClass("toast-success");
    // The project's icon, before the reference, has a letter of its own.
    expect(toast).toHaveTextContent(/^Added \S?o\/notes GitHub/);
    expect(toast).toHaveTextContent("The agent clones it and creates the state labels in about 10 seconds.");
    const section = screen.getByRole("region", { name: "Projects" });
    expect(section).toHaveTextContent("Run this on the relay's host to start sooner through a webhook. Until you do, the agent polls the project:");
    expect(within(section).getByText("switchboard relay hook add gh:o/notes").tagName).toBe("CODE");
  });

  it("show why an addition was refused by its code, keeping the server's message as it is", async () => {
    setLanguageChoice("en");
    answer({
      "GET /api/projects": projects(true, sandbox),
      "POST /api/projects": [
        { status: 409, body: { code: "project_exists", message: "exists" } },
        { status: 422, body: { code: "config_invalid", message: "設定の3行目" } },
      ],
    });
    at("/settings");
    const field = await screen.findByRole("textbox", { name: "Project to add" });
    await userEvent.type(field, "gh:o/r");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    const form = field.closest("form") as HTMLElement;
    expect(await within(form).findByRole("alert")).toHaveTextContent("This project is already in the config.");
    expect(field).toHaveValue("gh:o/r");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() =>
      expect(within(form).getByRole("alert")).toHaveTextContent("The project was not added because the config file has an error: 設定の3行目"),
    );
  });

  it("put a refusal shown into the language chosen afterwards", async () => {
    setLanguageChoice("en");
    answer({
      "GET /api/projects": projects(true, sandbox),
      "POST /api/projects": { status: 400, body: { code: "invalid", message: "bad" } },
    });
    at("/settings");
    const field = await screen.findByRole("textbox", { name: "Project to add" });
    await userEvent.type(field, "o/r");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    const form = field.closest("form") as HTMLElement;
    expect(await within(form).findByRole("alert")).toHaveTextContent("Enter gh:owner/repo or gl:group/project.");
    act(() => setLanguageChoice("ja"));
    expect(within(form).getByRole("alert")).toHaveTextContent("gh:owner/repo か gl:group/project の形で入力してください。");
  });

  it("say that the inbox service cannot be reached when the list cannot be read", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/projects": { status: 503, body: { code: "unavailable", message: "down" } } });
    at("/settings");
    const section = await screen.findByRole("region", { name: "Projects" });
    expect(await within(section).findByRole("alert")).toHaveTextContent("Cannot reach the inbox service (switchboard inbox serve).");
  });
});

const targets = {
  status: 200,
  body: {
    items: [
      { ref: "gh:o/r", host: "box", plan_first: true, plan_first_mode: "auto" },
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

/** openDialog opens the new issue's dialog in English from the list and waits for its projects. */
async function openDialog() {
  at("/");
  await userEvent.click(await screen.findByRole("button", { name: "New issue" }));
  const dialog = await screen.findByRole("dialog", { name: "New issue" });
  await waitFor(() => expect(within(dialog).getByRole("combobox", { name: "Project" })).toHaveTextContent("o/r GitHub (host: box)"));
  return dialog;
}

describe("a new issue in English", () => {
  it("is written in a dialog whose fields, starts and buttons are in English", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/issue-targets": targets });
    const dialog = await openDialog();
    expect(within(dialog).getByRole("heading", { name: "New issue" })).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("combobox", { name: "Project" }));
    const list = await within(dialog).findByRole("listbox");
    expect(within(list).getAllByRole("option").map((o) => o.getAttribute("aria-label"))).toEqual([
      "o/r GitHub (host: box)",
      "nas/sb GitLab (host: laptop)",
    ]);
    expect(within(list).getByRole("option", { name: "nas/sb GitLab (host: laptop)" })).toHaveTextContent("nas/sb GitLab (host: laptop)");
    await userEvent.keyboard("{Escape}");
    expect(within(dialog).getByRole("textbox", { name: "Title" })).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: "Description" })).toHaveAttribute("placeholder", "Markdown supported (optional)");
    const group = within(dialog).getByRole("group", { name: "Start label" });
    expect(within(group).getAllByRole("radio").map((r) => r.closest("label")?.textContent)).toEqual([
      "Follow the project's setting (agent:ready): start with a plan if needed",
      "Start with a plan (agent:plan)",
      "Implement without a plan (agent:go)",
      "No label (do not start)",
    ]);
    expect(within(group).getByRole("radio", { name: /^Follow the project's setting/ })).toBeChecked();
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeEnabled();
    expect(within(dialog).getByRole("button", { name: "Create" })).toBeDisabled();
    expect(dialog).not.toHaveTextContent(/[ぁ-んァ-ヶ一-龠（）：、・]/);
  });

  it("names each start of the ready label of the chosen project", async () => {
    setLanguageChoice("en");
    const items = [
      { ref: "gh:o/a", host: "box", plan_first: true, plan_first_mode: "always" },
      { ref: "gh:o/n", host: "box", plan_first: false, plan_first_mode: "never" },
      { ref: "gh:o/p", host: "box", plan_first: true },
    ];
    answer({ "GET /api/issue-targets": { status: 200, body: { ...targets.body, items } } });
    at("/");
    await userEvent.click(await screen.findByRole("button", { name: "New issue" }));
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    const ready = await within(dialog).findByRole("radio", { name: /^Follow the project's setting/ });
    expect(ready.closest("label")).toHaveTextContent("(agent:ready): always start with a plan");
    for (const [name, note] of [
      ["o/n GitHub (host: box)", "(agent:ready): implement right away"],
      ["o/p GitHub (host: box)", "(agent:ready): start with a plan"],
    ]) {
      await userEvent.click(within(dialog).getByRole("combobox", { name: "Project" }));
      await userEvent.click(await within(dialog).findByRole("option", { name }));
      await waitFor(() => expect(ready.closest("label")).toHaveTextContent(note));
    }
  });

  it("is created and told in an English toast with a link to the forge", async () => {
    setLanguageChoice("en");
    const calls = answer({
      "GET /api/issue-targets": targets,
      "POST /api/issues": { status: 201, body: { ref: "gl:nas/sb#7", url: "https://gitlab.test/nas/sb/-/issues/7" } },
    });
    const dialog = await openDialog();
    await userEvent.click(within(dialog).getByRole("combobox", { name: "Project" }));
    await userEvent.click(await within(dialog).findByRole("option", { name: "nas/sb GitLab (host: laptop)" }));
    await userEvent.type(within(dialog).getByRole("textbox", { name: "Title" }), "Add a flag");
    await userEvent.type(within(dialog).getByRole("textbox", { name: "Description" }), "Use --x.");
    await userEvent.click(within(dialog).getByRole("radio", { name: /agent:plan/ }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Create" }));

    const status = (await screen.findByText(/^Created/)).closest<HTMLElement>("[role=status]")!;
    expect(status).toHaveTextContent(/^Created \S?nas\/sb#7 GitLab/);
    expect(within(status).getByText("nas/sb#7").nextElementSibling).toHaveTextContent("GitLab");
    const toast = status.closest<HTMLElement>("[data-sonner-toast]")!;
    expect(toast).toHaveClass("toast-success");
    const link = within(toast).getByRole("link", { name: "Open on the forge" });
    expect(link).toHaveAttribute("href", "https://gitlab.test/nas/sb/-/issues/7");
    expect(screen.queryByRole("dialog")).toBeNull();
    const post = calls.find((c) => c.key === "POST /api/issues");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gl:nas/sb", title: "Add a flag", body: "Use --x.", start: "plan" });
  });

  it("shows why it was refused by its code, keeping the forge's message as it is", async () => {
    setLanguageChoice("en");
    answer({
      "GET /api/issue-targets": targets,
      "POST /api/issues": { status: 502, body: { code: "forge", message: "rate limited" } },
    });
    const dialog = await openDialog();
    await userEvent.type(within(dialog).getByRole("textbox", { name: "Title" }), "Add a flag");
    await userEvent.click(within(dialog).getByRole("button", { name: "Create" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("The forge could not create the issue: rate limited");
    expect(within(dialog).getByRole("textbox", { name: "Title" })).toHaveValue("Add a flag");
  });

  it("says that a body over the limit is too large", async () => {
    setLanguageChoice("en");
    answer({
      "GET /api/issue-targets": targets,
      "POST /api/issues": { status: 413, body: { code: "too_large", message: "body too large" } },
    });
    const dialog = await openDialog();
    await userEvent.type(within(dialog).getByRole("textbox", { name: "Title" }), "Big");
    await userEvent.click(within(dialog).getByRole("button", { name: "Create" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("The description is too large. Keep it to 1 MiB or less.");
  });
});
