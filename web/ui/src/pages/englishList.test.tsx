import { act, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { setLanguageChoice } from "@/lib/i18n";
import { baseOf, pendingTimeout, storageKey, type Mark } from "@/lib/pending";
import type { Item } from "@/lib/api";

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });

/** answer stubs fetch: the inbox answers with inbox(), anything else as not found. */
function answer(inbox: () => Response | Promise<Response>) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string) => {
      if (input === "/api/inbox") return inbox();
      return json({ code: "not_found", message: input }, 404);
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
const waiting = (ref: string, title: string, min: number, reason = "question"): Item => {
  const [project, number] = ref.slice(3).split("#");
  return {
    n: 1, state: "waiting", ref, project, number: Number(number), title, url: "https://forge.test/", host: "laptop",
    waiting_since: ago(min), reason, question: "?",
  };
};
const running: Item = { ...waiting("gh:a/b#5", "Runs", 0), n: undefined, state: "running", waiting_since: null, running_since: ago(5), progress: "working" };
const queued: Item = { ...waiting("gh:a/b#7", "Waits for a slot", 0), n: undefined, state: "queued", waiting_since: null, reason: "max-parallel", queued_since: ago(3) };
const done: Item = { ...waiting("gh:a/b#6", "Finished", 0), n: 2, state: "done", waiting_since: null, done_since: ago(60) };

const listing = (...items: Item[]) => () => json({ items, errors: [], fetched_at: ago(0) });

/** section is the list's section under a heading. */
const section = async (name: string) => (await screen.findByRole("heading", { level: 2, name })).closest("section") as HTMLElement;

/** cards is the card list of a section (the table holds the same items for another width). */
const cards = (s: HTMLElement) => s.querySelector(":scope > div:last-of-type") as HTMLElement;

/** shown is the text of an element without the parts hidden from assistive technology, such as the project's icon. */
const shown = (h: HTMLElement) => {
  const copy = h.cloneNode(true) as HTMLElement;
  copy.querySelectorAll("[aria-hidden='true']").forEach((n) => n.remove());
  return copy.textContent;
};

/** card is the link of the card that holds an issue's title. */
const card = (s: HTMLElement, title: string) => within(cards(s)).getByText(title).closest("a") as HTMLElement;

describe("the list in English", () => {
  it("heads each section in English", async () => {
    setLanguageChoice("en");
    answer(listing(waiting("gh:c/d#1", "Q", 5), running, queued, done));
    at("/");
    for (const name of ["Waiting", "Running", "Queued", "Done (waiting for close)"]) {
      expect(await screen.findByRole("heading", { level: 2, name })).toBeInTheDocument();
    }
    expect(within(await section("Done (waiting for close)")).getByText("Closed issues leave the list. Comment on one to resume the work.")).toBeInTheDocument();
  });

  it("counts a project's issues as one issue or as issues", async () => {
    setLanguageChoice("en");
    answer(listing(waiting("gh:c/d#1", "First in c/d", 30), waiting("gh:a/b#2", "Only in a/b", 20), waiting("gh:c/d#4", "Second in c/d", 10)));
    at("/");
    const s = cards(await section("Waiting"));
    expect(within(s).getAllByRole("heading", { level: 3 }).map(shown)).toEqual(["c/d GitHub 2 issues", "a/b GitHub 1 issue"]);
  });

  it("counts the sections and the waits by reason in the summary, one issue or issues", async () => {
    setLanguageChoice("en");
    answer(listing(waiting("gh:o/r#1", "Q1", 9), waiting("gh:o/r#2", "Q2", 8), waiting("gh:o/r#3", "C", 7, "ci"), running));
    at("/");
    const summary = await screen.findByRole("region", { name: "Inbox summary" });
    const count = (term: string) => within(summary).getByText(term).nextElementSibling?.textContent;
    expect(count("Waiting")).toBe("3 issues");
    expect(count("Running")).toBe("1 issue");
    expect(count("Queued")).toBe("0 issues");
    expect(count("Done")).toBe("0 issues");
    const rows = within(within(summary).getByRole("list", { name: "Reasons for waiting" })).getAllByRole("listitem");
    expect(rows[0]).toHaveTextContent("question2 issues");
    expect(rows[1]).toHaveTextContent("cifailed1 issue");
    expect(screen.getByRole("heading", { level: 2, name: "Waiting longest" })).toBeInTheDocument();
  });

  it("badges a ci wait with its word and failed, and a question with its word alone", async () => {
    setLanguageChoice("en");
    answer(listing(waiting("gh:o/r#1", "Asks", 9), waiting("gh:o/r#2", "Red", 8, "ci")));
    at("/");
    const s = await section("Waiting");
    expect(within(card(s, "Red")).getByText("ci").parentElement).toHaveTextContent(/^cifailed$/);
    expect(within(card(s, "Asks")).getByText("question").parentElement).toHaveTextContent(/^question$/);
  });

  it("says how long an issue has waited in English words", async () => {
    setLanguageChoice("en");
    answer(listing(waiting("gh:o/r#1", "Asks", 5)));
    at("/");
    const s = await section("Waiting");
    expect(within(card(s, "Asks")).getByText("waiting 5m")).toBeInTheDocument();
    const table = within(s).getByRole("table");
    expect(within(table).getAllByRole("columnheader").map((h) => h.textContent)).toEqual(["Issue", "Status", "Host", "Waiting"]);
    expect(within(table).getByText("5m ago")).toBeInTheDocument();
  });

  it("names the other states' rows and time columns in English", async () => {
    setLanguageChoice("en");
    answer(listing(running, queued, done));
    at("/");
    const heads = async (name: string) => within(within(await section(name)).getByRole("table")).getAllByRole("columnheader").map((h) => h.textContent);
    expect(await heads("Running")).toEqual(["Issue", "Status", "Host", "Elapsed"]);
    expect(await heads("Queued")).toEqual(["Issue", "Status", "Host", "Queued"]);
    expect(await heads("Done (waiting for close)")).toEqual(["Issue", "Status", "Host", "Done"]);
    expect(within(card(await section("Running"), "Runs")).getByText("Running")).toBeInTheDocument();
    expect(within(card(await section("Queued"), "Waits for a slot")).getByText("Queued")).toBeInTheDocument();
    expect(within(card(await section("Done (waiting for close)"), "Finished")).getByText("Done")).toBeInTheDocument();
  });

  it("says in English that nothing waits", async () => {
    setLanguageChoice("en");
    answer(listing());
    at("/");
    expect(await screen.findByText("No issues waiting")).toBeInTheDocument();
    expect(screen.getByText("Choose an issue from the list on the left")).toBeInTheDocument();
  });

  it("names the toggle, the list, its border and the keys in English", async () => {
    setLanguageChoice("en");
    answer(listing(waiting("gh:o/r#1", "Asks", 5)));
    at("/");
    const toggle = await screen.findByRole("button", { name: "Group by project" });
    expect(within(toggle).getByText("Group")).toHaveClass("sm:hidden");
    expect(screen.getByRole("navigation", { name: "Inbox" })).toBeInTheDocument();
    expect(screen.getByRole("separator", { name: "List width" })).toBeInTheDocument();
    expect(screen.getByText("j/k: move between waiting issues, r: jump to reply")).toBeInTheDocument();
  });

  it("says that it is loading in English", async () => {
    setLanguageChoice("en");
    answer(() => new Promise<Response>(() => {}));
    at("/");
    expect(await screen.findByRole("status", { name: "Loading" })).toHaveTextContent("Loading");
  });
});

describe("the inbox errors in English", () => {
  it("say that the inbox service is unreachable, and say it again in Japanese once the language changes", async () => {
    setLanguageChoice("en");
    answer(() => json({ code: "unavailable", message: "down" }, 503));
    at("/");
    expect(await screen.findByText("Cannot reach the inbox service (switchboard inbox serve). Check that the service is running.")).toBeInTheDocument();
    act(() => setLanguageChoice("ja"));
    expect(await screen.findByText(/受信箱のサービス（switchboard inbox serve）に届きません/)).toBeInTheDocument();
  });

  it("say why the listing could not be read, with the server's words as they are", async () => {
    setLanguageChoice("en");
    answer(() => json({ code: "internal", message: "boom" }, 500));
    at("/");
    expect(await screen.findByText("Could not load the inbox: boom")).toBeInTheDocument();
  });

  it("name a forge that could not be read", async () => {
    setLanguageChoice("en");
    answer(() => json({ items: [], errors: [{ forge: "GitHub", message: "rate limited" }], fetched_at: "" }));
    at("/");
    expect(await screen.findByText("Could not read GitHub: rate limited")).toBeInTheDocument();
  });
});

describe("a mark in English", () => {
  it("shows the row as processing, then a toast that the list does not show the action yet", async () => {
    setLanguageChoice("en");
    const it0 = waiting("gh:o/r#12", "Add a flag", 5);
    const mark: Mark = { ref: it0.ref, kind: "reply", at: Date.now() - pendingTimeout + 300, base: baseOf(it0) };
    sessionStorage.setItem(storageKey, JSON.stringify([mark]));
    answer(listing(it0));
    at("/");
    const s = await section("Running");
    expect(within(card(s, "Add a flag")).getByText("Processing")).toBeInTheDocument();
    expect(within(card(s, "Add a flag")).getByText("Waiting for the run to start")).toBeInTheDocument();
    const toast = (await screen.findByText(/does not show your action/, {}, { timeout: 2000 })).closest("[data-sonner-toast]") as HTMLElement;
    // The project's icon, hidden from assistive technology, puts its letter before the reference.
    expect(toast).toHaveTextContent(/^The list does not show your action on .*o\/r#12 GitHub yet$/);
    expect(within(toast).getByRole("button", { name: "Close" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: /Notifications/ })).toBeInTheDocument();
  });
});
