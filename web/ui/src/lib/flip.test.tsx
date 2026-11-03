import { render } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useFlip } from "@/lib/flip";

/** Rows draws each key as a row 40px high, in order; a key in hidden is drawn with no box, as CSS hides it. */
function Rows({ keys, hidden = [], ready = true }: { keys: string[]; hidden?: string[]; ready?: boolean }) {
  const root = useRef<HTMLDivElement>(null);
  useFlip(root, ready);
  return (
    <div ref={root} data-top="100">
      {keys.map((k, i) => (
        <div key={k} data-flip-key={k} data-top={hidden.includes(k) ? "" : String(100 + i * 40)} />
      ))}
    </div>
  );
}

let animate: ReturnType<typeof vi.fn>;

beforeEach(() => {
  animate = vi.fn();
  // jsdom lays nothing out: a box is read from data-top, and an element without one has no box.
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    const top = this.dataset.top;
    return top ? ({ top: Number(top), left: 0, width: 300, height: 40 } as DOMRect) : ({ top: 0, left: 0, width: 0, height: 0 } as DOMRect);
  });
  Object.defineProperty(HTMLElement.prototype, "animate", { value: animate, configurable: true, writable: true });
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  delete (HTMLElement.prototype as Partial<HTMLElement>).animate;
});

const calls = () =>
  animate.mock.calls.map(([frames], i) => ({ key: (animate.mock.contexts[i] as HTMLElement).dataset.flipKey, frames }));

describe("useFlip", () => {
  it("moves a row from where it was to where it is", () => {
    const { rerender } = render(<Rows keys={["a", "b"]} />);
    expect(animate).not.toHaveBeenCalled();
    rerender(<Rows keys={["b", "a"]} />);
    expect(calls()).toEqual([
      { key: "b", frames: [{ transform: "translate(0px, 40px)" }, { transform: "none" }] },
      { key: "a", frames: [{ transform: "translate(0px, -40px)" }, { transform: "none" }] },
    ]);
  });

  it("fades in a row that was not there", () => {
    const { rerender } = render(<Rows keys={["a"]} />);
    rerender(<Rows keys={["a", "c"]} />);
    expect(calls()).toEqual([{ key: "c", frames: [{ opacity: 0 }, { opacity: 1 }] }]);
  });

  it("leaves rows that did not move", () => {
    const { rerender } = render(<Rows keys={["a", "b"]} />);
    rerender(<Rows keys={["a", "b"]} />);
    expect(animate).not.toHaveBeenCalled();
  });

  it("skips the copy of a row that CSS hides", () => {
    const { rerender } = render(<Rows keys={["a", "b"]} hidden={["a"]} />);
    rerender(<Rows keys={["b", "a"]} hidden={["a"]} />);
    expect(calls().map((c) => c.key)).toEqual(["b"]);
  });

  it("fades nothing in on the first listing", () => {
    const { rerender } = render(<Rows keys={[]} ready={false} />);
    rerender(<Rows keys={["a", "b"]} />);
    expect(animate).not.toHaveBeenCalled();
  });

  it("does nothing when less motion is asked for", () => {
    vi.stubGlobal("matchMedia", (q: string) => ({ matches: q.includes("reduce"), media: q }));
    const { rerender } = render(<Rows keys={["a", "b"]} />);
    rerender(<Rows keys={["b", "a", "c"]} />);
    expect(animate).not.toHaveBeenCalled();
  });
});
