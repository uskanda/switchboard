import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { setLanguageChoice } from "@/lib/i18n";

type Answer = { status: number; body?: unknown };

/** answer stubs fetch with routes by "METHOD path", and records every call with its body. */
function answer(routes: Record<string, Answer | (() => Answer)>) {
  const calls: { key: string; body?: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, body: init?.body ? JSON.parse(init.body as string) : undefined });
      const r = routes[key];
      const a = typeof r === "function" ? r() : r;
      if (a) return new Response(a.body === undefined ? null : JSON.stringify(a.body), { status: a.status });
      if (key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      if (key === "GET /api/projects") return new Response(JSON.stringify({ host: "ubuntu-a", enabled: false, items: [] }), { status: 200 });
      if (key === "GET /api/instructions") return new Response(JSON.stringify({ host: "ubuntu-a", items: [] }), { status: 200 });
      return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
    }),
  );
  return calls;
}

afterEach(() => {
  vi.unstubAllGlobals();
  setLanguageChoice("auto");
});

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const issuePath = "/issue/gh%3Ao%2Fr%2312";
const detailKey = "GET /api/issue?ref=gh%3Ao%2Fr%2312";
const grantsKey = "GET /api/permissions?issue=gh%3Ao%2Fr%2312";

const permissionWait = (denials: unknown[]) => ({
  status: 200,
  body: {
    ref: "gh:o/r#12", title: "Renumber", url: "https://github.test/o/r/issues/12", waiting: true,
    host: "laptop", state: "waiting", reason: "permission", since: null, approve_word: "/ok",
    post: "### The agent was refused an operation", denials,
  },
});

const gitMv = { tool: "Bash", command: "git mv a.sql b.sql", candidates: ["Bash(git mv)", "Bash(git mv *)"] };
const loop = { tool: "Bash", command: "for f in a b; do git log -- $f; done", candidates: [], compound: true };
const listed = { n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "Renumber", url: "u", host: "laptop", waiting_since: null, reason: "permission", question: "" };
const noGrants = { status: 200, body: { host: "ubuntu-a", items: [{ scope: "global", rules: [] }, { scope: "issue", ref: "gh:o/r#12", rules: [] }] } };

describe("the refused operations of a permission wait", () => {
  it("grant the rule for this issue, then ask the agent to run again", async () => {
    const calls = answer({
      "GET /api/inbox": { status: 200, body: { items: [listed], errors: [], fetched_at: "" } },
      [detailKey]: permissionWait([gitMv, loop]),
      [grantsKey]: noGrants,
      "POST /api/permissions": { status: 204 },
      "POST /api/reply": { status: 204 },
    });
    at(issuePath);
    const box = await screen.findByRole("region", { name: "拒否された操作" });
    expect(within(box).getByText("git mv a.sql b.sql")).toBeInTheDocument();
    expect(within(box).getByRole("textbox", { name: "git mv a.sql b.sql を許可する規則" })).toHaveValue("Bash(git mv *)");
    expect(within(box).getByText(/規則では許可できません/)).toBeInTheDocument();
    expect(within(box).getByRole("radio", { name: "このissue" })).toBeChecked();
    expect(screen.queryByRole("button", { name: "再実行する" })).toBeNull();
    await userEvent.click(within(box).getByRole("button", { name: "許可して再実行" }));
    expect(await screen.findByText(/送りました/)).toBeInTheDocument();
    const posts = calls.filter((c) => c.key.startsWith("POST"));
    expect(posts.map((c) => c.key)).toEqual(["POST /api/permissions", "POST /api/reply"]);
    expect(posts[0].body).toEqual({ scope: "issue", ref: "gh:o/r#12", add: ["Bash(git mv)", "Bash(git mv *)"] });
    expect(posts[1].body).toMatchObject({ ref: "gh:o/r#12", text: "再実行してください。" });
  });

  it("grant an edited rule to the project without running again", async () => {
    const calls = answer({ [detailKey]: permissionWait([gitMv]), [grantsKey]: noGrants, "POST /api/permissions": { status: 204 } });
    at(issuePath);
    const box = await screen.findByRole("region", { name: "拒否された操作" });
    const rule = within(box).getByRole("textbox", { name: "git mv a.sql b.sql を許可する規則" });
    await userEvent.clear(rule);
    await userEvent.type(rule, "Bash(git mv a.sql *)");
    await userEvent.click(within(box).getByRole("radio", { name: "このプロジェクト" }));
    await userEvent.click(within(box).getByRole("button", { name: "許可だけ" }));
    expect(await within(box).findByText("許可しました")).toBeInTheDocument();
    const posts = calls.filter((c) => c.key.startsWith("POST"));
    expect(posts.map((c) => c.key)).toEqual(["POST /api/permissions"]);
    expect(posts[0].body).toEqual({ scope: "project", ref: "gh:o/r", add: ["Bash(git mv a.sql *)"] });
  });

  it("cannot grant a command joined with others", async () => {
    answer({ [detailKey]: permissionWait([loop]), [grantsKey]: noGrants });
    at(issuePath);
    const box = await screen.findByRole("region", { name: "拒否された操作" });
    expect(within(box).getByRole("button", { name: "許可して再実行" })).toBeDisabled();
    expect(within(box).getByRole("button", { name: "許可だけ" })).toBeDisabled();
    expect(screen.getByRole("textbox", { name: "返答" })).toBeEnabled();
  });

  it("show why a rule was refused, and run nothing again", async () => {
    const calls = answer({
      [detailKey]: permissionWait([gitMv]),
      [grantsKey]: noGrants,
      "POST /api/permissions": { status: 422, body: { code: "rule_refused", message: "refused", rules: [{ rule: "Bash(git *)", reason: "allows every git command, outside the sandbox" }] } },
    });
    at(issuePath);
    const box = await screen.findByRole("region", { name: "拒否された操作" });
    const rule = within(box).getByRole("textbox", { name: "git mv a.sql b.sql を許可する規則" });
    await userEvent.clear(rule);
    await userEvent.type(rule, "Bash(git *)");
    await userEvent.click(within(box).getByRole("button", { name: "許可して再実行" }));
    expect(await within(box).findByText(/allows every git command, outside the sandbox/)).toBeInTheDocument();
    expect(calls.some((c) => c.key === "POST /api/reply")).toBe(false);
  });

  it("does not ask to run again when the save fails", async () => {
    const calls = answer({ [detailKey]: permissionWait([gitMv]), [grantsKey]: noGrants, "POST /api/permissions": { status: 502, body: { code: "forge", message: "disk full" } } });
    at(issuePath);
    const box = await screen.findByRole("region", { name: "拒否された操作" });
    await userEvent.click(within(box).getByRole("button", { name: "許可して再実行" }));
    expect(await within(box).findByText(/disk full/)).toBeInTheDocument();
    expect(calls.some((c) => c.key === "POST /api/reply")).toBe(false);
  });

  it("is in English", async () => {
    setLanguageChoice("en");
    answer({ [detailKey]: permissionWait([gitMv, loop]), [grantsKey]: noGrants });
    at(issuePath);
    const box = await screen.findByRole("region", { name: "Refused operations" });
    expect(within(box).getByRole("textbox", { name: "Rule that allows git mv a.sql b.sql" })).toBeInTheDocument();
    expect(within(box).getByRole("radio", { name: "This issue" })).toBeChecked();
    expect(within(box).getByRole("button", { name: "Allow and run again" })).toBeInTheDocument();
    expect(within(box).getByText(/No rule can allow/)).toBeInTheDocument();
  });

  it("only lists the operations to someone who is not an admin", async () => {
    const calls = answer({ "GET /api/auth": { status: 200, body: { enabled: true, user: "bob", admin: false } }, [detailKey]: permissionWait([gitMv]) });
    at(issuePath);
    const box = await screen.findByRole("region", { name: "拒否された操作" });
    expect(within(box).getByText("git mv a.sql b.sql")).toBeInTheDocument();
    expect(within(box).queryByRole("button", { name: "許可して再実行" })).toBeNull();
    expect(within(box).queryByRole("textbox")).toBeNull();
    expect(calls.some((c) => c.key.startsWith("GET /api/permissions"))).toBe(false);
  });
});

describe("the granted rules", () => {
  it("of the issue are listed on its page and can be removed", async () => {
    const calls = answer({
      [detailKey]: permissionWait([]),
      [grantsKey]: { status: 200, body: { host: "ubuntu-a", items: [{ scope: "global", rules: [] }, { scope: "issue", ref: "gh:o/r#12", rules: ["Bash(git ls-tree *)"] }] } },
      "POST /api/permissions": { status: 204 },
    });
    at(issuePath);
    const list = await screen.findByRole("region", { name: "このissueの許可" });
    await userEvent.click(within(list).getByRole("button", { name: "Bash(git ls-tree *) を外す" }));
    const post = calls.find((c) => c.key === "POST /api/permissions");
    expect(post?.body).toEqual({ scope: "issue", ref: "gh:o/r#12", remove: ["Bash(git ls-tree *)"] });
  });

  it("of every project and of each project are edited on the settings page", async () => {
    const calls = answer({
      "GET /api/permissions": {
        status: 200,
        body: { host: "ubuntu-a", items: [{ scope: "global", rules: ["Skill"] }, { scope: "project", ref: "gh:o/r", rules: [] }] },
      },
      "POST /api/permissions": { status: 204 },
    });
    at("/settings");
    const section = await screen.findByRole("region", { name: "許可" });
    expect(await within(section).findByText("ubuntu-a")).toBeInTheDocument();
    expect(within(section).getByText("Skill")).toBeInTheDocument();
    await userEvent.type(within(section).getByRole("textbox", { name: "gh:o/r に足す規則" }), "Bash(git blame *)");
    await userEvent.click(within(section).getByRole("button", { name: "gh:o/r に足す" }));
    await userEvent.click(within(section).getByRole("button", { name: "Skill を外す" }));
    const posts = calls.filter((c) => c.key === "POST /api/permissions").map((c) => c.body);
    expect(posts).toEqual([
      { scope: "project", ref: "gh:o/r", add: ["Bash(git blame *)"] },
      { scope: "global", remove: ["Skill"] },
    ]);
  });

  it("are not read for someone who is not an admin", async () => {
    const calls = answer({ "GET /api/auth": { status: 200, body: { enabled: true, user: "bob", admin: false } } });
    at("/settings");
    expect(await screen.findByText(/管理者だけです/)).toBeInTheDocument();
    expect(calls.some((c) => c.key.startsWith("GET /api/permissions"))).toBe(false);
    expect(screen.queryByRole("region", { name: "許可" })).toBeNull();
  });
});
