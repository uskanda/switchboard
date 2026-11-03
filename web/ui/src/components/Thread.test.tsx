import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Thread } from "@/components/Thread";
import type { Comment, Detail } from "@/lib/api";

const agent = (m: number, kind: string, body = kind): Comment => ({ author: "bot", created_at: `2026-10-31T10:0${m}:00Z`, body, agent: true, kind });
const person = (m: number, body: string): Comment => ({ author: "me", created_at: `2026-10-31T10:0${m}:00Z`, body, agent: false });
const detail = (comments: Comment[], ref = "gh:o/r#1"): Detail => ({
  ref, title: "t", url: "u", waiting: true, host: "h", state: "waiting", reason: "plan-questions", since: null, post: "", body: "本文", comments,
});

/** Two answered rounds, the first answered once and the second not, and the latest question. */
const comments = [
  agent(1, "waiting", "最初の質問"), person(2, "最初の返答"), agent(3, "running"),
  agent(4, "waiting", "二つ目の質問"), agent(5, "running"),
  agent(6, "waiting", "最新の質問"),
];

describe("Thread", () => {
  it("folds the answered rounds to a line each and leaves the latest question open and marked", () => {
    render(<Thread detail={detail(comments)} />);
    const summaries = screen.getAllByText(/答えた質問/);
    expect(summaries.map((s) => s.textContent)).toEqual(["答えた質問・返答1件", "答えた質問・返答なし"]);
    expect(screen.getByText("最初の質問")).not.toBeVisible();
    expect(screen.getByText("最初の返答")).not.toBeVisible();
    const latest = screen.getByText("最新の質問");
    expect(latest).toBeVisible();
    expect(latest.closest("[aria-current]")).toHaveAttribute("aria-current", "true");
    expect(screen.getAllByText("着手しました")).toHaveLength(2);
  });

  it("opens a round on pressing its line, and keeps it open through a read of the same issue", () => {
    const { rerender } = render(<Thread key="gh:o/r#1" detail={detail(comments)} />);
    fireEvent.click(screen.getByText("答えた質問・返答1件"));
    expect(screen.getByText("最初の返答")).toBeVisible();
    rerender(<Thread key="gh:o/r#1" detail={detail([...comments, person(7, "新しい返答")])} />);
    expect(screen.getByText("最初の返答")).toBeVisible();
    rerender(<Thread key="gh:o/r#2" detail={detail(comments, "gh:o/r#2")} />);
    expect(screen.getByText("最初の返答")).not.toBeVisible();
  });

  it("keeps the comment's index on the cards inside a round", () => {
    const { container } = render(<Thread detail={detail(comments)} />);
    expect(container.querySelector('details [data-index="1"]')).toHaveTextContent("最初の返答");
  });
});
