import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Markdown } from "@/components/Markdown";

describe("Markdown", () => {
  it("draws a single line break inside a paragraph as a line break", () => {
    const { container } = render(<Markdown text={"A: 右クリックだけ\nB: 選択メニューだけ"} />);
    const p = container.querySelector("p")!;
    expect(p.querySelectorAll("br")).toHaveLength(1);
    expect(p.innerHTML.replace(/\n/g, "")).toBe("A: 右クリックだけ<br>B: 選択メニューだけ");
  });

  it("leaves the line breaks inside code as they are", () => {
    const { container } = render(<Markdown text={"```\na\nb\n```"} />);
    expect(container.querySelector("br")).toBeNull();
    expect(container.querySelector("code")).toHaveTextContent(/^a\s*b$/);
  });

  it("breaks the lines of raw HTML shown as text too", () => {
    const { container } = render(<Markdown text={"<b>x</b>\n次の行"} />);
    expect(container.querySelector("b")).toBeNull();
    expect(container.querySelectorAll("br")).toHaveLength(1);
  });

  it("draws headings two levels down, under the page's own", () => {
    const { container } = render(<Markdown text={"# a\n\n## b\n\n### c\n\n#### d\n\n###### e"} />);
    const tags = [...container.querySelectorAll("h1, h2, h3, h4, h5, h6")].map((h) => `${h.tagName}:${h.textContent}`);
    expect(tags).toEqual(["H3:a", "H4:b", "H5:c", "H6:d", "H6:e"]);
  });
});
