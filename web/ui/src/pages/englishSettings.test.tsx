import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { LANGUAGE_KEY, setLanguageChoice } from "@/lib/i18n";

// The settings, the login, the page that does not exist and the header in English, and the choice of the language
// on the settings page and on the login page (openspec/changes/add-english-support, task 2.4).

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
  fail() {
    act(() => this.onerror?.());
  }
}

type Answer = { status: number; body?: unknown };

/** answer makes fetch reply by "METHOD path" and records the keys asked; the list and the projects are empty unless given. */
function answer(routes: Record<string, Answer>) {
  const asked: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      asked.push(key);
      const a = routes[key];
      if (!a && key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      if (!a && key === "GET /api/projects") return new Response(JSON.stringify({ host: "desk", enabled: false, items: [] }), { status: 200 });
      if (!a) return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
      return new Response(a.status === 204 ? null : JSON.stringify(a.body), { status: a.status });
    }),
  );
  return asked;
}

function setBrowserLanguages(languages: string[]) {
  Object.defineProperty(window.navigator, "languages", { value: languages, configurable: true });
}

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
  setBrowserLanguages(["ja-JP", "ja"]);
});

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const instructions = {
  host: "desk",
  items: [
    { target: "", kind: "plan", content: "" },
    { target: "gh:o/r", kind: "plan", content: "" },
    { target: "", kind: "run", content: "" },
    { target: "", kind: "implement", content: "" },
  ],
};

const nonAdmin = { "GET /api/auth": { status: 200, body: { enabled: true, user: "bob", admin: false } } };

describe("the choice of the language on the settings page", () => {
  it("shows the page in English at once when English is chosen, and keeps the choice", async () => {
    answer({ "GET /api/instructions": { status: 200, body: instructions } });
    at("/settings");
    const display = await screen.findByRole("group", { name: "言語" });
    expect(within(display).getByRole("radio", { name: "自動" })).toBeChecked();
    expect(within(display).getByText("English")).toHaveAttribute("lang", "en");
    expect(within(display).getByText("日本語")).toHaveAttribute("lang", "ja");

    await userEvent.click(within(display).getByRole("radio", { name: "English" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Settings" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Display" })).toBeInTheDocument();
    const english = screen.getByRole("group", { name: "Language" });
    expect(within(english).getByRole("radio", { name: "English" })).toBeChecked();
    expect(within(english).getByRole("radio", { name: "Automatic" })).not.toBeChecked();
    expect(within(english).getByText("日本語")).toHaveAttribute("lang", "ja");
    expect(document.documentElement.lang).toBe("en");
    expect(localStorage.getItem(LANGUAGE_KEY)).toBe("en");
    await waitFor(() => expect(document.title).toBe("Settings - SwitchBoard Inbox"));
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "/settings");
  });

  it("keeps the choice in the browser and sends nothing to the server", async () => {
    const asked = answer({ "GET /api/instructions": { status: 200, body: instructions } });
    at("/settings");
    await screen.findByRole("heading", { name: "計画の実行への追加の指示" });
    await waitFor(() => expect(asked).toContain("GET /api/projects"));
    const before = vi.mocked(fetch).mock.calls.length;

    await userEvent.click(screen.getByRole("radio", { name: "English" }));
    await screen.findByRole("heading", { level: 1, name: "Settings" });
    await userEvent.click(screen.getByRole("radio", { name: "日本語" }));
    await screen.findByRole("heading", { level: 1, name: "設定" });
    await userEvent.click(screen.getByRole("radio", { name: "自動" }));

    expect(vi.mocked(fetch).mock.calls.length).toBe(before);
    for (const [url, init] of vi.mocked(fetch).mock.calls) {
      expect(String(url)).not.toMatch(/lang/i);
      expect(String(init?.body ?? "")).not.toMatch(/lang|"(ja|en|auto)"/i);
    }
  });

  it("puts the display section first, above the projects and the instructions", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/instructions": { status: 200, body: instructions } });
    at("/settings");
    const display = await screen.findByRole("heading", { name: "Display" });
    const plan = await screen.findByRole("heading", { name: "Additional instructions for plan runs" });
    const projects = document.getElementById("projects-heading")!;
    expect(display.compareDocumentPosition(projects) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(display.compareDocumentPosition(plan) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("follows the browser's languages again when Automatic is chosen", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/instructions": { status: 200, body: instructions } });
    at("/settings");
    await userEvent.click(await screen.findByRole("radio", { name: "Automatic" }));
    expect(await screen.findByRole("heading", { level: 1, name: "設定" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "自動" })).toBeChecked();
    expect(document.documentElement.lang).toBe("ja");
    expect(localStorage.getItem(LANGUAGE_KEY)).toBeNull();
  });

  it("shows English under Automatic when the browser prefers English to Japanese", async () => {
    setBrowserLanguages(["fr-FR", "en", "ja"]);
    setLanguageChoice("ja");
    answer({ "GET /api/instructions": { status: 200, body: instructions } });
    at("/settings");
    expect(await screen.findByRole("radio", { name: "日本語" })).toBeChecked();
    await userEvent.click(screen.getByRole("radio", { name: "自動" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Settings" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Automatic" })).toBeChecked();
    expect(document.documentElement.lang).toBe("en");
  });
});

describe("the settings page in English", () => {
  it("shows the instructions, the host they are on and the command for another host", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/instructions": { status: 200, body: instructions } });
    at("/settings");
    const plan = (await screen.findByRole("heading", { name: "Additional instructions for plan runs" })).closest("section")!;
    expect(within(plan).getByRole("textbox", { name: "All projects" })).toBeInTheDocument();
    expect(within(plan).getByText(/These files are on host/)).toHaveTextContent(
      "These files are on host desk. If the agent runs on another host, use switchboard instructions edit there.",
    );
    expect(screen.getByRole("heading", { name: "Additional instructions for implementation runs" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Additional instructions for implementation runs only" })).toBeInTheDocument();
    // The projects section is another part's; the display section and the instructions are in English throughout.
    const japanese = /[ぁ-んァ-ン一-龥]/;
    for (const part of document.querySelectorAll("section[data-kind], section[aria-labelledby='display-heading']")) {
      expect(part.textContent?.replace("日本語", "")).not.toMatch(japanese);
    }
    expect(document.querySelectorAll("section[data-kind]")).toHaveLength(3);
  });

  it("says a change is not saved, then that it was saved, in a toast that names the field", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/instructions": { status: 200, body: instructions }, "POST /api/instructions": { status: 204 } });
    at("/settings");
    const plan = (await screen.findByRole("heading", { name: "Additional instructions for plan runs" })).closest("section")!;
    const field = within(plan).getByRole("textbox", { name: "All projects" });
    await userEvent.type(field, "Ask first.");
    const card = field.closest("section")!;
    expect(within(card).getByText("Unsaved changes")).toBeInTheDocument();
    await userEvent.click(within(card).getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Saved the plan instructions for all projects");
  });

  it("names the project of a field in the toast", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/instructions": { status: 200, body: instructions }, "POST /api/instructions": { status: 204 } });
    at("/settings");
    const plan = (await screen.findByRole("heading", { name: "Additional instructions for plan runs" })).closest("section")!;
    const field = within(plan).getByRole("textbox", { name: "o/r GitHub" });
    await userEvent.type(field, "Ask first.");
    await userEvent.click(within(field.closest("section")!).getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Saved the plan instructions for o/r GitHub");
  });

  it("explains a refusal in English in the field", async () => {
    setLanguageChoice("en");
    answer({
      "GET /api/instructions": { status: 200, body: instructions },
      "POST /api/instructions": { status: 422, body: { code: "secret", message: "looks like a token" } },
    });
    at("/settings");
    const plan = (await screen.findByRole("heading", { name: "Additional instructions for plan runs" })).closest("section")!;
    const field = within(plan).getByRole("textbox", { name: "All projects" });
    await userEvent.type(field, "ghp_x");
    const card = field.closest("section")!;
    await userEvent.click(within(card).getByRole("button", { name: "Save" }));
    expect(await within(card).findByRole("alert")).toHaveTextContent("Not saved: it seems to contain a secret.");
  });

  it("shows someone not an admin the display section and that only an admin can change the rest, and reads nothing", async () => {
    setLanguageChoice("en");
    const asked = answer(nonAdmin);
    at("/settings");
    expect(await screen.findByText("Only an admin can change the projects and additional instructions.")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Display" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "English" })).toBeChecked();
    expect(asked).not.toContain("GET /api/instructions");
    expect(asked).not.toContain("GET /api/projects");
  });

  it("lets someone not an admin open it from the header and choose the language", async () => {
    setLanguageChoice("en");
    answer(nonAdmin);
    at("/");
    expect(await screen.findByText("bob")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("link", { name: "Settings" }));
    await userEvent.click(await screen.findByRole("radio", { name: "日本語" }));
    expect(await screen.findByText("プロジェクトと追加の指示を変えられるのは管理者だけです。")).toBeInTheDocument();
    expect(localStorage.getItem(LANGUAGE_KEY)).toBe("ja");
  });
});

describe("the login page", () => {
  const loggedOut = { "GET /api/auth": { status: 200, body: { enabled: true } } };

  it("switches from English to Japanese and back with the button in the other language", async () => {
    setLanguageChoice("en");
    answer(loggedOut);
    at("/");
    expect(await screen.findByRole("link", { name: "Log in with GitLab" })).toHaveAttribute("href", "/auth/login");
    expect(screen.getByRole("heading", { name: "Log in" })).toBeInTheDocument();
    await waitFor(() => expect(document.title).toBe("Log in - SwitchBoard Inbox"));
    const toJapanese = screen.getByRole("button", { name: "日本語" });
    expect(toJapanese).toHaveAttribute("lang", "ja");

    await userEvent.click(toJapanese);
    expect(await screen.findByRole("link", { name: "GitLabでログイン" })).toBeInTheDocument();
    expect(localStorage.getItem(LANGUAGE_KEY)).toBe("ja");
    expect(document.documentElement.lang).toBe("ja");
    const toEnglish = screen.getByRole("button", { name: "English" });
    expect(toEnglish).toHaveAttribute("lang", "en");
    expect(screen.queryByRole("button", { name: "日本語" })).toBeNull();

    await userEvent.click(toEnglish);
    expect(await screen.findByRole("link", { name: "Log in with GitLab" })).toBeInTheDocument();
    expect(localStorage.getItem(LANGUAGE_KEY)).toBe("en");
  });

  it("says in English why the last login did not go through", async () => {
    setLanguageChoice("en");
    answer(loggedOut);
    at("/login?error=not_allowed&user=mallory");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "GitLab user mallory has no access to this inbox. Log in as another user, or ask an admin for access.",
    );
  });

  it("says in English that GitLab denied the login", async () => {
    setLanguageChoice("en");
    answer(loggedOut);
    at("/login?error=denied");
    expect(await screen.findByRole("alert")).toHaveTextContent("GitLab denied the login.");
  });
});

describe("the header in English", () => {
  const base = { project: "o/r", url: "https://github.test/o/r/issues/1", question: "?", reason: "", host: "desk" };
  const t0 = new Date(Date.now() - 5 * 60_000).toISOString();
  const waiting = (n: number) => ({ ...base, n, state: "waiting", ref: `gh:o/r#${n}`, number: n, title: `Waits ${n}`, reason: "question", waiting_since: t0 });
  const running = (n: number) => ({ ...base, state: "running", ref: `gh:o/r#${n}`, number: n, title: `Runs ${n}`, running_since: t0, progress: "p" });
  const queued = { ...base, state: "queued", ref: "gh:o/r#40", number: 40, title: "Queued work", reason: "max-parallel", queued_since: t0 };
  const done = { ...base, n: 9, state: "done", ref: "gh:o/r#50", number: 50, title: "Finished", done_since: t0, change_url: "https://github.test/o/r/pull/9" };

  it("counts one waiting issue in the singular", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": { status: 200, body: { items: [waiting(1), running(30), running(31)], errors: [], fetched_at: "" } } });
    at("/");
    const header = within(await screen.findByRole("banner"));
    expect(await header.findByText("1 issue waiting, 2 running")).toBeInTheDocument();
    expect(header.getByText("1 waiting")).toHaveClass("text-attention");
    expect(header.getByText("1 waiting").parentElement).toHaveTextContent("1 waiting, 2 running");
  });

  it("counts several waiting issues in the plural, with the queued and done ones", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": { status: 200, body: { items: [waiting(1), waiting(2), running(30), queued, done], errors: [], fetched_at: "" } } });
    at("/");
    const header = within(await screen.findByRole("banner"));
    expect(await header.findByText("2 issues waiting, 1 running, 1 queued, 1 done")).toBeInTheDocument();
    expect(header.getByText("2 waiting").parentElement).toHaveTextContent("2 waiting, 1 running, 1 queued, 1 done");
  });

  it("counts no waiting issue in the plural", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": { status: 200, body: { items: [running(30)], errors: [], fetched_at: "" } } });
    at("/");
    const header = within(await screen.findByRole("banner"));
    expect(await header.findByText("0 issues waiting, 1 running")).toBeInTheDocument();
  });

  it("shows the settings link to someone logged in who is not an admin, and the logout", async () => {
    setLanguageChoice("en");
    answer(nonAdmin);
    at("/");
    expect(await screen.findByText("bob")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "/settings");
    expect(screen.getByRole("button", { name: "Log out" })).toBeInTheDocument();
  });

  it("says in English when the logout fails", async () => {
    setLanguageChoice("en");
    answer({ ...nonAdmin, "POST /api/logout": { status: 503, body: { code: "unavailable", message: "down" } } });
    at("/");
    await userEvent.click(await screen.findByRole("button", { name: "Log out" }));
    expect(await screen.findByText("Could not log out")).toBeInTheDocument();
    expect(screen.getByText("bob")).toBeInTheDocument();
  });

  describe("with the stream", () => {
    beforeEach(() => {
      FakeEventSource.last = null;
      vi.stubGlobal("EventSource", FakeEventSource);
    });

    it("says the state of the live updates in English", async () => {
      setLanguageChoice("en");
      answer({});
      at("/");
      const es = FakeEventSource.last!;
      expect(screen.getByText("Connecting to live updates")).toBeInTheDocument();
      es.emit("inbox", { items: [], errors: [], fetched_at: t0, relay: "connected", refresh_interval_seconds: 30 });
      expect(await screen.findByText("Live updates on (instant)")).toBeInTheDocument();
      es.emit("inbox", { items: [], errors: [], fetched_at: t0, relay: "disconnected", refresh_interval_seconds: 30 });
      expect(await screen.findByText("Live updates on (every 30s)")).toBeInTheDocument();
      es.emit("inbox", { items: [], errors: [], fetched_at: t0 });
      expect(await screen.findByText("Live updates on")).toBeInTheDocument();
    });

    it("says when the list is from while the updates are cut", async () => {
      const cut = new Date();
      cut.setSeconds(0, 0);
      vi.useFakeTimers({ toFake: ["Date"] });
      vi.setSystemTime(cut);
      try {
        setLanguageChoice("en");
        answer({});
        at("/");
        const es = FakeEventSource.last!;
        es.emit("inbox", { items: [], errors: [], fetched_at: new Date(cut.getTime() - 60 * 60_000).toISOString() });
        await screen.findByText("Live updates on");
        es.fail();
        expect(await screen.findByText("Live updates lost. Reconnecting")).toBeInTheDocument();
        const time = document.querySelector("header time")!;
        const hhmm = `${String(cut.getHours()).padStart(2, "0")}:${String(cut.getMinutes()).padStart(2, "0")}`;
        expect(time.parentElement).toHaveTextContent(`List as of ${hhmm}`);
      } finally {
        vi.useRealTimers();
      }
    });
  });
});

describe("the page that does not exist in English", () => {
  it("says so, links back to the inbox and names the tab", async () => {
    setLanguageChoice("en");
    answer({});
    at("/nowhere");
    expect(await screen.findByRole("heading", { name: "Page not found" })).toBeInTheDocument();
    expect(screen.getByText("No page matches this URL. It may have a typo, or the link may be out of date.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to inbox" })).toHaveAttribute("href", "/");
    await waitFor(() => expect(document.title).toBe("Page not found - SwitchBoard Inbox"));
  });
});
