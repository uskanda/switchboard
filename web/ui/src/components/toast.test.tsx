import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Toaster } from "@/components/ui/toaster";
import { toast } from "@/lib/toast";

/** toasts lists the toasts on the screen, oldest last as sonner orders them. */
const toasts = () => Array.from(document.querySelectorAll<HTMLElement>("[data-sonner-toast]"));

/** wait lets the fake clock run, and with it sonner's own timers for mounting and removing. */
/** It runs in steps so that what sonner schedules after a render (removing a closed toast) runs in the same wait. */
async function wait(ms: number) {
  const step = 50;
  for (let done = 0; done < ms || done === 0; done += step) {
    await act(async () => vi.advanceTimersByTime(Math.min(step, ms - done)));
  }
}

// sonner marks a dismissed toast in an animation frame, so the frames run on the fake clock too.
beforeEach(() => vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval", "Date", "requestAnimationFrame", "cancelAnimationFrame"] }));
afterEach(() => vi.useRealTimers());

describe("a toast", () => {
  it("shows each kind with its own class and role, an error as an alert", async () => {
    render(<Toaster />);
    act(() => {
      toast.info("info message");
      toast.success("success message");
      toast.warning("warning message");
      toast.error("error message");
    });
    await wait(0);
    const byText = (text: string) => toasts().find((t) => t.textContent?.includes(text))!;
    expect(byText("info message")).toHaveClass("toast-info");
    expect(byText("success message")).toHaveClass("toast-success");
    expect(byText("warning message")).toHaveClass("toast-warning");
    expect(byText("error message")).toHaveClass("toast-error");
    expect(within(byText("error message")).getByRole("alert")).toHaveTextContent("error message");
    for (const text of ["info message", "success message", "warning message"]) {
      expect(within(byText(text)).getByRole("status")).toHaveTextContent(text);
      expect(within(byText(text)).queryByRole("alert")).toBeNull();
    }
  });

  it("carries a description and an action", async () => {
    render(<Toaster />);
    act(() => {
      toast.success("gh:o/r#1 を作りました", { description: "more", action: <a href="https://x.test/1">open</a> });
    });
    await wait(0);
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("gh:o/r#1 を作りました");
    expect(status).toHaveTextContent("more");
    expect(within(toasts()[0]).getByRole("link", { name: "open" })).toHaveAttribute("href", "https://x.test/1");
  });

  it("is closed by its close button", async () => {
    render(<Toaster />);
    act(() => {
      toast.info("close me");
    });
    await wait(0);
    fireEvent.click(screen.getByRole("button", { name: "閉じる" }));
    await wait(500);
    expect(screen.queryByText("close me")).toBeNull();
  });

  it("goes away after 5 seconds, or after its own duration", async () => {
    render(<Toaster />);
    act(() => {
      toast.info("five");
      toast.info("two", { duration: 2000 });
    });
    await wait(1999);
    expect(screen.getByText("two")).toBeInTheDocument();
    await wait(500);
    expect(screen.queryByText("two")).toBeNull();
    expect(screen.getByText("five")).toBeInTheDocument();
    await wait(2500);
    expect(screen.getByText("five")).toBeInTheDocument();
    await wait(1000);
    expect(screen.queryByText("five")).toBeNull();
  });

  it("stays while the pointer is over it", async () => {
    render(<Toaster />);
    act(() => {
      toast.info("hovered");
    });
    await wait(1000);
    fireEvent.mouseOver(toasts()[0]);
    await wait(10000);
    expect(screen.getByText("hovered")).toBeInTheDocument();
    fireEvent.mouseOut(toasts()[0], { relatedTarget: document.body });
    await wait(3999);
    expect(screen.getByText("hovered")).toBeInTheDocument();
    await wait(1500);
    expect(screen.queryByText("hovered")).toBeNull();
  });

  it("stays while the focus is in it", async () => {
    render(
      <>
        <button type="button">outside</button>
        <Toaster />
      </>,
    );
    act(() => {
      toast.info("focused");
    });
    await wait(1000);
    act(() => screen.getByRole("button", { name: "閉じる" }).focus());
    await wait(10000);
    expect(screen.getByText("focused")).toBeInTheDocument();
    act(() => screen.getByRole("button", { name: "outside" }).focus());
    await wait(3999);
    expect(screen.getByText("focused")).toBeInTheDocument();
    await wait(1500);
    expect(screen.queryByText("focused")).toBeNull();
  });

  it("shows at most five, dropping the oldest", async () => {
    render(<Toaster />);
    act(() => {
      for (let i = 1; i <= 6; i++) toast.info(`toast ${i}`);
    });
    await wait(500);
    expect(toasts()).toHaveLength(5);
    expect(screen.queryByText("toast 1")).toBeNull();
    expect(screen.getByText("toast 6")).toBeInTheDocument();
  });
});
