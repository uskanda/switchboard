import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

// jsdom has no PointerEvent, and the plain Event it falls back to drops clientX; a MouseEvent with a pointerId will do.
if (typeof window.PointerEvent === "undefined") {
  class FakePointerEvent extends MouseEvent {
    readonly pointerId: number;
    constructor(type: string, init: PointerEventInit = {}) {
      super(type, init);
      this.pointerId = init.pointerId ?? 0;
    }
  }
  window.PointerEvent = FakePointerEvent as unknown as typeof PointerEvent;
}

beforeEach(() => {
  window.localStorage.clear();
  vi.stubGlobal("innerWidth", 1440);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 })),
  );
});

afterEach(() => vi.unstubAllGlobals());

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

/** listWidth is the width the layout gives the list pane, as the CSS variable on the two-pane grid. */
function listWidth() {
  return screen.getByRole("navigation", { name: "一覧" }).parentElement!.style.getPropertyValue("--list-width");
}

describe("the list pane's border", () => {
  it("starts at 360px", () => {
    at("/");
    const sep = screen.getByRole("separator", { name: "一覧の幅" });
    expect(sep).toHaveAttribute("aria-orientation", "vertical");
    expect(sep).toHaveAttribute("aria-valuenow", "360");
    expect(listWidth()).toBe("360px");
  });

  it("moves with the arrow keys and is remembered", async () => {
    at("/");
    const sep = screen.getByRole("separator", { name: "一覧の幅" });
    sep.focus();
    await userEvent.keyboard("{ArrowRight}{ArrowRight}");
    expect(sep).toHaveAttribute("aria-valuenow", "392");
    expect(listWidth()).toBe("392px");
    await userEvent.keyboard("{ArrowLeft}");
    expect(listWidth()).toBe("376px");
    expect(window.localStorage.getItem("switchboard.listWidth")).toBe("376");
  });

  it("goes to the ends with Home and End", async () => {
    at("/");
    const sep = screen.getByRole("separator", { name: "一覧の幅" });
    sep.focus();
    await userEvent.keyboard("{Home}");
    expect(listWidth()).toBe("280px");
    await userEvent.keyboard("{End}");
    expect(listWidth()).toBe("640px");
  });

  it("follows a drag and stops at the minimum", () => {
    at("/");
    const sep = screen.getByRole("separator", { name: "一覧の幅" });
    fireEvent.pointerDown(sep, { clientX: 360, pointerId: 1, button: 0 });
    fireEvent.pointerMove(sep, { clientX: 460, pointerId: 1 });
    expect(listWidth()).toBe("460px");
    fireEvent.pointerMove(sep, { clientX: 100, pointerId: 1 });
    expect(listWidth()).toBe("280px");
    fireEvent.pointerUp(sep, { clientX: 100, pointerId: 1 });
    fireEvent.pointerMove(sep, { clientX: 500, pointerId: 1 });
    expect(listWidth()).toBe("280px");
  });

  it("goes back to 360px on a double click", async () => {
    window.localStorage.setItem("switchboard.listWidth", "500");
    at("/");
    expect(listWidth()).toBe("500px");
    await userEvent.dblClick(screen.getByRole("separator", { name: "一覧の幅" }));
    expect(listWidth()).toBe("360px");
  });

  it("keeps the issue pane its room on a narrow window", () => {
    window.localStorage.setItem("switchboard.listWidth", "640");
    vi.stubGlobal("innerWidth", 1024);
    at("/");
    expect(listWidth()).toBe("544px");
  });
});
