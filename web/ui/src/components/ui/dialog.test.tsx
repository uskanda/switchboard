import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Dialog } from "@/components/ui/dialog";

function List({ order }: { order: string[] }) {
  return (
    <ul>
      {order.map((name) => (
        <li key={name}>
          {name}
          <Dialog aria-label={`dialog ${name}`}>
            <p>{name}</p>
          </Dialog>
        </li>
      ))}
    </ul>
  );
}

describe("a dialog", () => {
  // A browser takes a modal dialog out of the top layer when an ancestor is moved, and leaves it open but not modal:
  // hidden in its row, Escape no longer closes it, and showModal throws, so its button does nothing (a merge button
  // after the listing reordered the rows). The dialog therefore lives outside the part of the page that moves.
  it("stays where it is when its row moves", () => {
    const { rerender } = render(<List order={["a", "b"]} />);
    const dialog = screen.getByLabelText("dialog b", { selector: "dialog" });
    const parent = dialog.parentNode;
    expect(dialog.closest("li")).toBeNull();
    rerender(<List order={["b", "a"]} />);
    expect(screen.getByLabelText("dialog b", { selector: "dialog" })).toBe(dialog);
    expect(dialog.parentNode).toBe(parent);
    expect(dialog.isConnected).toBe(true);
  });
});
