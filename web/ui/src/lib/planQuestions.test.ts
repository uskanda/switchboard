import { describe, expect, it } from "vitest";
import { smarternews80 } from "@/lib/planQuestions.fixture";
import { composeReply, defaultAnswers, groupsOf, parsePlanQuestions, withoutAnswerHint, type Question } from "@/lib/planQuestions";

const labels = (q: Question) => groupsOf(q).map((g) => g.options.map((o) => o.label).join(""));

/** question builds one question comment from the body and recommendation of a single question. */
const question = (body: string, rec = "A。") => `### 計画についての質問\n\n❓ **Q1** - **題**\n\n${body}\n\n➡️ ${rec}\n`;

const parsedOne = (body: string, rec?: string) => parsePlanQuestions(question(body, rec))!.questions[0];

describe("parsePlanQuestions", () => {
  it("reads every question of a comment with its number, title, body and recommendation", () => {
    const p = parsePlanQuestions(smarternews80)!;
    expect(p.questions.map((q) => q.n)).toEqual([1, 2, 3, 4, 5, 6]);
    expect(p.questions[0].title).toBe("操作の入口");
    expect(p.questions[2].title).toBe("ダイアログの既定値");
    expect(p.questions[0].recommendation).toMatch(/^C。issueの要望は右クリック/);
    expect(p.head).toMatch(/^### 計画についての質問/);
    expect(p.head).toMatch(/feed-filters/);
  });

  it("leaves the hint at the end out of the last recommendation", () => {
    const p = parsePlanQuestions(smarternews80)!;
    const last = p.questions[5];
    expect(last.recommendation).toMatch(/^A。読んでいる途中で/);
    expect(last.recommendation).not.toMatch(/推奨どおり/);
    expect(p.tail).toMatch(/^推奨どおりでよければ/);
  });

  it("keeps the recommendation of a question that is not the last whole, up to the separator", () => {
    const body = "### 計画についての質問\n\n❓ **Q1** - **一**\n\nA: x\nB: y\n\n➡️ A。\n\n理由の続き。\n\n---\n\n❓ **Q2** - **二**\n\n本文\n\n➡️ B。\n";
    const p = parsePlanQuestions(body)!;
    expect(p.questions[0].recommendation).toBe("A。\n\n理由の続き。");
    expect(p.questions[1].recommendation).toBe("B。");
  });

  it("returns null for a comment without questions", () => {
    expect(parsePlanQuestions("### エージェントからの質問\n\nどちらにしますか。")).toBeNull();
  });

  it("accepts the variants of the heading line", () => {
    const p = parsePlanQuestions("❓️ **Q7** – **題**: 本文の始まり\n\n➡️ A\n")!;
    expect(p.questions[0].n).toBe(7);
    expect(p.questions[0].title).toBe("題");
    expect(p.questions[0].parts).toEqual([{ kind: "text", text: "本文の始まり" }]);
  });
});

describe("the groups of options", () => {
  it("are read from options on lines of their own", () => {
    const q = parsePlanQuestions(smarternews80)!.questions[0];
    expect(labels(q)).toEqual(["ABC"]);
    expect(groupsOf(q)[0].options[2]).toEqual({ label: "C", text: "AとBの両方。" });
    expect(q.parts[0]).toEqual({ kind: "text", text: "選んだ文字列からフィルターを作る操作を、どこに置きますか。" });
  });

  it("are read from one line split by slashes, each with its heading", () => {
    const q = parsePlanQuestions(smarternews80)!.questions[2];
    expect(labels(q)).toEqual(["ABC", "XY"]);
    expect(groupsOf(q).map((g) => g.heading)).toEqual(["範囲", "欄"]);
    expect(groupsOf(q)[1].options[1]).toEqual({ label: "Y", text: "タイトルだけ" });
  });

  it("keep the text after the options as text", () => {
    const q = parsePlanQuestions(smarternews80)!.questions[3];
    expect(labels(q)).toEqual(["ABC"]);
    expect(q.parts.at(-1)).toEqual({ kind: "text", text: expect.stringMatching(/^どれでも、生成記事では出しません/) });
  });

  it.each([
    ["- **A**: 一つ目\n- **B**: 二つ目", "AB"],
    ["**A.** 一つ目\n**B.** 二つ目", "AB"],
    ["(A) 一つ目\n(B) 二つ目", "AB"],
    ["A) 一つ目\nB) 二つ目", "AB"],
    ["A：一つ目\nB：二つ目", "AB"],
    ["A、一つ目\nB、二つ目", "AB"],
    ["A 一つ目\nB 二つ目", "AB"],
    ["1. 一つ目\n2. 二つ目\n3. 三つ目", "123"],
    ["A: 一つ目\n\nB: 二つ目", "AB"],
    ["A 全体 ／ B 購読フィード", "AB"],
    ["A: 右 / B: 左", "AB"],
  ])("accept the variant %j", (body, want) => {
    expect(labels(parsedOne(body))).toEqual([want]);
  });

  it("take a line ending in a colon just above as the heading", () => {
    const q = parsedOne("前置き。\n範囲：\nA: 全体\nB: 購読フィード");
    expect(groupsOf(q)[0].heading).toBe("範囲");
    expect(q.parts[0]).toEqual({ kind: "text", text: "前置き。" });
  });

  it("join an indented line to the option above it", () => {
    const q = parsedOne("- A: 一つ目\n  続きの行\n- B: 二つ目");
    expect(groupsOf(q)[0].options[0].text).toBe("一つ目 続きの行");
  });

  it.each([
    ["labels that do not count up", "A user opens the page.\nI think it is fine."],
    ["a single labeled line", "A: 一つだけ"],
    ["labels that skip", "A: 一つ目\nC: 三つ目"],
    ["letters mixed with digits", "A: 一つ目\n2: 二つ目"],
    ["lines in a code block", "```\nA: 一つ目\nB: 二つ目\n```"],
  ])("are not read from %s", (_, body) => {
    expect(groupsOf(parsedOne(body))).toEqual([]);
  });
});

describe("defaultAnswers", () => {
  const defaults = (body: string, rec: string) => {
    const q = parsedOne(body, rec);
    return defaultAnswers([q])[0].choices;
  };

  it("choose the first label of each group that the recommendation names", () => {
    const qs = parsePlanQuestions(smarternews80)!.questions;
    expect(defaultAnswers(qs).map((a) => a.choices)).toEqual([["C"], ["A"], ["A", "X"], ["A"], ["A"], ["A"]]);
  });

  it("do not count a label that is part of a word or a number", () => {
    expect(defaults("A: x\nB: y", "APIの名前を変え、Bにする")).toEqual(["B"]);
    expect(defaults("1. x\n2. y", "200文字までにして、2を選ぶ")).toEqual(["2"]);
  });

  it("choose none when the recommendation names no label of the group", () => {
    expect(defaults("A: x\nB: y", "APIの名前は今のままにします。")).toEqual([null]);
  });

  it("start with no note", () => {
    expect(defaultAnswers([parsedOne("本文")])[0]).toEqual({ choices: [], note: "" });
  });
});

describe("composeReply", () => {
  const qs = parsePlanQuestions(smarternews80)!.questions;

  it("writes one line per question with its number, title and answer", () => {
    const answers = defaultAnswers(qs);
    answers[1] = { choices: ["B"], note: "件数は出さない" };
    const lines = composeReply(qs, answers, "").split("\n");
    expect(lines[0]).toBe("Q1 操作の入口: C");
    expect(lines[1]).toBe("Q2 選んだあとの流れ: B。補足：件数は出さない");
    expect(lines[2]).toBe("Q3 ダイアログの既定値: 範囲はA、欄はX");
    expect(lines).toHaveLength(6);
  });

  it("joins the groups without headings with commas", () => {
    const q = parsedOne("A: x\nB: y\n\n中の文\n\n1. p\n2. q");
    expect(composeReply([q], [{ choices: ["B", "1"], note: "" }], "")).toBe("Q1 題: B、1");
  });

  it("leaves out a group with no choice", () => {
    expect(composeReply([qs[2]], [{ choices: [null, "Y"], note: "" }], "")).toBe("Q3 ダイアログの既定値: 欄はY");
  });

  it("answers with the note alone when nothing is chosen", () => {
    expect(composeReply([qs[0]], [{ choices: [null], note: "  どれでもなく、長押しにする \n" }], "")).toBe("Q1 操作の入口: どれでもなく、長押しにする");
  });

  it("writes 回答なし for a question with neither a choice nor a note", () => {
    expect(composeReply([qs[4]], [{ choices: [null], note: " " }], "")).toBe("Q5 Androidアプリの扱い: 回答なし");
  });

  it("keeps a note on one line", () => {
    expect(composeReply([qs[0]], [{ choices: ["A"], note: "一行目\n二行目" }], "")).toBe("Q1 操作の入口: A。補足：一行目 二行目");
  });

  it("adds the text of the reply field as a note on the whole", () => {
    const text = composeReply([qs[0]], [{ choices: ["C"], note: "" }], "  全体として急がない\n二行目 ");
    expect(text).toBe("Q1 操作の入口: C\n\n全体への補足：\n全体として急がない\n二行目");
  });

  it("never starts with the approval word", () => {
    expect(composeReply(qs, defaultAnswers(qs), "/ok")).toMatch(/^Q1 /);
  });
});

describe("withoutAnswerHint", () => {
  it("drops the hint on the approval word in either language and keeps the rest", () => {
    const tail = "変更を捨てました。\n\n推奨どおりでよければ `/ok`、変えたい問いがあれば `/ok q2はB` のように返答してください。";
    expect(withoutAnswerHint(tail)).toBe("変更を捨てました。");
    expect(withoutAnswerHint("Reply `/ok` to accept the recommended answers, or `/ok q2 is B` to change some of them.")).toBe("");
  });
});
