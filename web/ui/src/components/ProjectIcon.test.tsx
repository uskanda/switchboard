import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { IssueRef } from "@/components/IssueRef";

function emoji(icons: Record<string, string> | null) {
  const fetch = vi.fn(async (input: string) => {
    if (input === "/api/project-icons" && icons) return new Response(JSON.stringify(icons), { status: 200 });
    return new Response(JSON.stringify({ code: "not_found", message: input }), { status: 404 });
  });
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

afterEach(() => vi.unstubAllGlobals());

const icon = (container: HTMLElement) => container.querySelector("[data-project-icon]") as HTMLElement;

describe("the project's icon", () => {
  it("is the image of the project before the reference, with an empty alt", async () => {
    emoji({});
    const { container } = render(<IssueRef value="gh:o/r#12" />);
    const img = container.querySelector("img")!;
    expect(img).toHaveAttribute("src", "/api/project-icon?ref=gh%3Ao%2Fr");
    expect(img).toHaveAttribute("alt", "");
    expect(img).toHaveAttribute("width", "16");
    expect(icon(container).compareDocumentPosition(screen.getByText("o/r#12")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // Nothing of the icon is read out: the name stays the reference and the forge.
    expect(icon(container)).toHaveAttribute("aria-hidden", "true");
    fireEvent.load(img);
    expect(icon(container)).not.toHaveTextContent("R");
  });

  it("is the project's initial while the image loads, and when it fails", async () => {
    emoji({});
    const { container } = render(<IssueRef value="gl:nas/sandbox" />);
    expect(icon(container)).toHaveTextContent("S");
    fireEvent.error(container.querySelector("img")!);
    expect(container.querySelector("img")).toBeNull();
    expect(icon(container)).toHaveTextContent("S");
  });

  it("is the emoji of the configuration, without an image", async () => {
    emoji({ "gh:o/r": "🚀" });
    const { container } = render(<IssueRef value="gh:o/r#12" />);
    await waitFor(() => expect(icon(container)).toHaveTextContent("🚀"));
    expect(container.querySelector("img")).toBeNull();
  });

  it("reads the emoji once for every reference on the page", async () => {
    const fetch = emoji({});
    render(
      <>
        <IssueRef value="gh:o/r#1" />
        <IssueRef value="gh:o/r#2" />
        <IssueRef value="gl:nas/sb" />
      </>,
    );
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(fetch.mock.calls.filter(([u]) => u === "/api/project-icons")).toHaveLength(1);
  });

  it("is left out of a value that is not a reference", () => {
    emoji(null);
    const { container } = render(<IssueRef value="すべてのプロジェクト" />);
    expect(icon(container)).toBeNull();
  });
});
