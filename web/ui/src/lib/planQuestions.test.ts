import { describe, expect, it } from "vitest";
import { smarternews80 } from "@/lib/planQuestions.fixture";
import { composeReply, decidedBefore, defaultAnswers, groupsOf, parsePlanQuestions, questionState, setNoteOnly, splitHead, withoutAnswerHint, withoutRecommendedMark, type Question } from "@/lib/planQuestions";

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
    const lines = composeReply("ja", qs, answers, "").split("\n");
    expect(lines[0]).toBe("Q1 操作の入口: C");
    expect(lines[1]).toBe("Q2 選んだあとの流れ: B。補足：件数は出さない");
    expect(lines[2]).toBe("Q3 ダイアログの既定値: 範囲はA、欄はX");
    expect(lines).toHaveLength(6);
  });

  it("joins the groups without headings with commas", () => {
    const q = parsedOne("A: x\nB: y\n\n中の文\n\n1. p\n2. q");
    expect(composeReply("ja", [q], [{ choices: ["B", "1"], note: "" }], "")).toBe("Q1 題: B、1");
  });

  it("leaves out a group with no choice", () => {
    expect(composeReply("ja", [qs[2]], [{ choices: [null, "Y"], note: "" }], "")).toBe("Q3 ダイアログの既定値: 欄はY");
  });

  it("answers with the note alone when nothing is chosen", () => {
    expect(composeReply("ja", [qs[0]], [{ choices: [null], note: "  どれでもなく、長押しにする \n" }], "")).toBe("Q1 操作の入口: どれでもなく、長押しにする");
  });

  it("writes 回答なし for a question with neither a choice nor a note", () => {
    expect(composeReply("ja", [qs[4]], [{ choices: [null], note: " " }], "")).toBe("Q5 Androidアプリの扱い: 回答なし");
  });

  it("keeps a note on one line", () => {
    expect(composeReply("ja", [qs[0]], [{ choices: ["A"], note: "一行目\n二行目" }], "")).toBe("Q1 操作の入口: A。補足：一行目 二行目");
  });

  it("adds the text of the reply field as a note on the whole", () => {
    const text = composeReply("ja", [qs[0]], [{ choices: ["C"], note: "" }], "  全体として急がない\n二行目 ");
    expect(text).toBe("Q1 操作の入口: C\n\n全体への補足：\n全体として急がない\n二行目");
  });

  it("never starts with the approval word", () => {
    expect(composeReply("ja", qs, defaultAnswers(qs), "/ok")).toMatch(/^Q1 /);
  });
});

describe("withoutAnswerHint", () => {
  it("drops the hint on the approval word in either language and keeps the rest", () => {
    const tail = "変更を捨てました。\n\n推奨どおりでよければ `/ok`、変えたい問いがあれば `/ok q2はB` のように返答してください。";
    expect(withoutAnswerHint(tail)).toBe("変更を捨てました。");
    expect(withoutAnswerHint("Reply `/ok` to accept the recommended answers, or `/ok q2 is B` to change some of them.")).toBe("");
  });
});

describe("splitHead", () => {
  it("shows the heading and the plan's title line, and folds what the agent looked at", () => {
    const head = "### 計画についての質問\n\n**計画：** 題の行\n\n`a.go` と `b.go` を読みました。\n\n今は真偽値です。";
    expect(splitHead(head)).toEqual({ shown: "### 計画についての質問\n\n**計画：** 題の行", folded: "`a.go` と `b.go` を読みました。\n\n今は真偽値です。" });
  });

  it("finds the title line in English", () => {
    const head = "### The agent has questions for the plan\n\n**Plan:** A title\n\nI read a.go.";
    expect(splitHead(head)).toEqual({ shown: "### The agent has questions for the plan\n\n**Plan:** A title", folded: "I read a.go." });
  });

  it("folds nothing without a title line", () => {
    const head = "### 計画についての質問\n\n`a.go` を読みました。";
    expect(splitHead(head)).toEqual({ shown: head, folded: "" });
  });

  it("folds nothing when nothing follows the title line", () => {
    expect(splitHead("### 計画についての質問\n\n**計画：** 題")).toEqual({ shown: "### 計画についての質問\n\n**計画：** 題", folded: "" });
  });
});

describe("setNoteOnly", () => {
  it("clears every choice when it is set, and gives the choices back when it is cleared", () => {
    const on = setNoteOnly({ choices: ["B", "X"], note: "" }, true);
    expect(on.noteOnly).toBe(true);
    expect(on.choices).toEqual([null, null]);
    const off = setNoteOnly(on, false);
    expect(off.noteOnly).toBe(false);
    expect(off.choices).toEqual(["B", "X"]);
  });

  it("keeps the note either way", () => {
    expect(setNoteOnly({ choices: ["A"], note: "メモ" }, true).note).toBe("メモ");
  });

  it("writes the note alone, or 回答なし, for a question answered in the note alone", () => {
    const q = parsedOne("A: x\nB: y");
    expect(composeReply("ja", [q], [setNoteOnly({ choices: ["A"], note: "どちらも要らない" }, true)], "")).toBe("Q1 題: どちらも要らない");
    expect(composeReply("ja", [q], [setNoteOnly({ choices: ["A"], note: "" }, true)], "")).toBe("Q1 題: 回答なし");
  });
});

describe("questionState", () => {
  const q = parsedOne("A: x\nB: y");
  const two = parsedOne("範囲: A 全体 / B 購読\n欄: X 両方 / Y 題だけ", "範囲はA、欄はX。");
  const unlabeled = parsedOne("A: x\nB: y", "名前は今のままにします。");
  const plain = parsedOne("どう呼びますか。", "今の名前のまま。");

  it("is recommended while every recommended group keeps its recommended option", () => {
    expect(questionState(q, { choices: ["A"], note: "" })).toEqual({ state: "recommended", noted: false });
    expect(questionState(two, { choices: ["A", "X"], note: "" })).toEqual({ state: "recommended", noted: false });
  });

  it("is changed when a recommended group has another option", () => {
    expect(questionState(q, { choices: ["B"], note: "" })).toEqual({ state: "changed", noted: false });
    expect(questionState(two, { choices: ["A", "Y"], note: "" })).toEqual({ state: "changed", noted: false });
  });

  it("marks a note on top of the state", () => {
    expect(questionState(q, { choices: ["A"], note: "急がない" })).toEqual({ state: "recommended", noted: true });
  });

  it("is note only with the box ticked, and for a question without groups that has a note", () => {
    expect(questionState(q, setNoteOnly({ choices: ["A"], note: "x" }, true))).toEqual({ state: "noteOnly", noted: false });
    expect(questionState(plain, { choices: [], note: "x" })).toEqual({ state: "noteOnly", noted: false });
  });

  it("is unanswered with a group left without a choice and no note, or no groups and no note", () => {
    expect(questionState(unlabeled, { choices: [null], note: "" })).toEqual({ state: "unanswered", noted: false });
    expect(questionState(plain, { choices: [], note: " " })).toEqual({ state: "unanswered", noted: false });
    expect(questionState(q, setNoteOnly({ choices: ["A"], note: "" }, true))).toEqual({ state: "unanswered", noted: false });
  });

  it("is chosen when no group has a recommended option", () => {
    expect(questionState(unlabeled, { choices: ["B"], note: "" })).toEqual({ state: "chosen", noted: false });
  });
});

describe("decidedBefore", () => {
  type C = { author: string; created_at: string; body: string; agent: boolean; kind?: string };
  /** round builds a question comment of the agent's from questions given as [n, title, options, recommendation]. */
  const round = (...qs: [number, string, string, string][]): C => ({
    author: "bot",
    created_at: "",
    agent: true,
    kind: "waiting",
    body: `### 計画についての質問\n\n**計画：** 題\n\n${qs.map(([n, t, b, r]) => `❓ **Q${n}** - **${t}**\n\n${b}\n\n➡️ ${r}`).join("\n\n---\n\n")}\n`,
  });
  const person = (body: string): C => ({ author: "me", created_at: "", body, agent: false });
  const claim: C = { author: "bot", created_at: "", body: "着手", agent: true, kind: "running" };
  const q1: [number, string, string, string] = [1, "範囲", "A: 全体\nB: 一部", "Aを勧めます。"];
  const q2: [number, string, string, string] = [2, "名前: 呼び方", "A: 今のまま\nB: 変える", "Bを勧めます。"];
  const q3: [number, string, string, string] = [3, "順番", "A: 古い順\nB: 新しい順", "Aを勧めます。"];

  it("lists the answers of the earlier rounds of the cycle, with the text of a single chosen option", () => {
    const cs = [person("issue"), claim, round(q1, q2), person("Q1 範囲: B\nQ2 名前: 呼び方: A。補足：急がない"), claim, round(q3)];
    expect(decidedBefore("ja", cs, 5, "/ok")).toEqual([
      { kind: "question", n: 1, title: "範囲", answer: "B 一部" },
      { kind: "question", n: 2, title: "名前: 呼び方", answer: "A 今のまま。補足：急がない" },
    ]);
  });

  it("takes the last line of a question when there are several", () => {
    const cs = [round(q1), person("Q1 範囲: A"), person("Q1 範囲: B"), claim, round(q3)];
    expect(decidedBefore("ja", cs, 4, "/ok")).toEqual([{ kind: "question", n: 1, title: "範囲", answer: "B 一部" }]);
  });

  it("takes the recommended options for a round answered with the approval word alone", () => {
    const cs = [round(q1, q2), person(" /ok \n"), claim, round(q3)];
    expect(decidedBefore("ja", cs, 3, "/ok")).toEqual([
      { kind: "question", n: 1, title: "範囲", answer: "A 全体" },
      { kind: "question", n: 2, title: "名前: 呼び方", answer: "B 変える" },
    ]);
  });

  it("says a question without a recommended option was answered as recommended", () => {
    const cs = [round([1, "名前", "A: x\nB: y", "今のままにします。"]), person("/ok"), claim, round(q3)];
    expect(decidedBefore("ja", cs, 3, "/ok")).toEqual([{ kind: "question", n: 1, title: "名前", answer: "推奨どおり" }]);
    expect(decidedBefore("en", cs, 3, "/ok")[0]).toMatchObject({ answer: "As recommended" });
  });

  it("gives the first line of the reply for a round answered in words", () => {
    const cs = [round(q1, q2), person("/ok q2はB\nあとは任せます"), claim, round(q3)];
    expect(decidedBefore("ja", cs, 3, "/ok")).toEqual([{ kind: "reply", from: 1, to: 2, text: "/ok q2はB" }]);
  });

  it("starts over at the latest round that begins with Q1", () => {
    const cs = [round(q1), person("Q1 範囲: A"), claim, round(q1, q2), person("Q1 範囲: B\nQ2 名前: 呼び方: B"), claim, round(q3)];
    expect(decidedBefore("ja", cs, 6, "/ok").map((d) => d.kind === "question" && d.answer)).toEqual(["B 一部", "B 変える"]);
  });

  it("lists nothing for the first round", () => {
    const cs = [round(q1), person("Q1 範囲: A"), claim, round(q1, q2)];
    expect(decidedBefore("ja", cs, 3, "/ok")).toEqual([]);
  });

  it("keeps an answer of two groups as written", () => {
    const two: [number, string, string, string] = [1, "既定", "範囲: A 全体 / B 一部\n欄: X 両方 / Y 題", "範囲はA、欄はX。"];
    const cs = [round(two), person("Q1 既定: 範囲はB、欄はX"), claim, round(q3)];
    expect(decidedBefore("ja", cs, 3, "/ok")).toEqual([{ kind: "question", n: 1, title: "既定", answer: "範囲はB、欄はX" }]);
  });
});

describe("the diagram of a question", () => {
  const body = (q: string) => `### 計画についての質問\n\n${q}`;
  const kinds = (q: Question) => q.parts.map((p) => p.kind);

  it("is the first mermaid code block before the options", () => {
    const q = parsePlanQuestions(body("❓ **Q1** - **流れ**\n\nどこで分けますか。\n\n```mermaid\nflowchart TD\n  A --> B:::focus\n```\n\nA: 今のまま\nB: 分ける\n\n➡️ **推奨：B** 速い。\n"))!.questions[0];
    expect(kinds(q)).toEqual(["text", "diagram", "group"]);
    expect(q.parts[1]).toEqual({ kind: "diagram", source: "flowchart TD\n  A --> B:::focus" });
    expect(defaultAnswers([q])[0].choices).toEqual(["B"]);
  });

  it("is not a code block of another language, nor one after the options", () => {
    const q = parsePlanQuestions(body("❓ **Q1** - **流れ**\n\n```go\nx := 1\n```\n\nA: 今のまま\nB: 分ける\n\n```mermaid\nflowchart TD\n  A --> B\n```\n\n➡️ A\n"))!.questions[0];
    expect(kinds(q)).toEqual(["text", "group", "text"]);
  });

  it("keeps an unclosed mermaid block as text", () => {
    const q = parsePlanQuestions(body("❓ **Q1** - **流れ**\n\n```mermaid\nflowchart TD\n  A --> B\n\n➡️ A\n"))!.questions[0];
    expect(kinds(q)).toEqual(["text"]);
    expect((q.parts[0] as { text: string }).text).toMatch(/^```mermaid\nflowchart TD/);
  });
});

describe("the images of a question", () => {
  const body = (q: string) => `### 計画についての質問\n\n${q}`;
  const kinds = (q: Question) => q.parts.map((p) => p.kind);
  // As internal/agent/planrun.go writes a question with images on GitLab.
  const written =
    "❓ **Q1** - **配置**\n\n保存のボタンをどこに置きますか。\n\n![q.png](/uploads/0123456789abcdef0123456789abcdef/q.png)\n\n" +
    "A: 今のまま\nB: 上に移す\n![b.png](/uploads/fedcba9876543210fedcba9876543210/b.png)\nC: 消す\n\n➡️ **推奨：B** 最初に目に入る。\n";

  it("reads the question's image before the options and an option's image after its line", () => {
    const q = parsePlanQuestions(body(written))!.questions[0];
    expect(kinds(q)).toEqual(["text", "image", "group"]);
    expect(q.parts[1]).toEqual({ kind: "image", src: "/uploads/0123456789abcdef0123456789abcdef/q.png", alt: "q.png" });
    const [g] = groupsOf(q);
    expect(g.options.map((o) => o.label)).toEqual(["A", "B", "C"]);
    expect(g.options[1]).toEqual({ label: "B", text: "上に移す", image: { src: "/uploads/fedcba9876543210fedcba9876543210/b.png", alt: "b.png" } });
    expect(g.options[0].image).toBeUndefined();
    expect(defaultAnswers([q])[0].choices).toEqual(["B"]);
  });

  it("keeps an image after the options, or one among text, as text", () => {
    const q = parsePlanQuestions(body("❓ **Q1** - **配置**\n\nA: 今のまま\nB: 上に移す\n\n![x](/uploads/0123456789abcdef0123456789abcdef/x.png)\n\n➡️ A\n"))!.questions[0];
    expect(kinds(q)).toEqual(["group", "text"]);
    const t = parsePlanQuestions(body("❓ **Q1** - **配置**\n\n説明 ![x](/uploads/a/x.png) です。\n\nA: 今のまま\nB: 上に移す\n\n➡️ A\n"))!.questions[0];
    expect(kinds(t)).toEqual(["text", "group"]);
  });
});

describe("withoutRecommendedMark", () => {
  it("drops the bold label at the start of a recommendation, in either language", () => {
    expect(withoutRecommendedMark("**推奨：B** 読み込みが速くなる")).toBe("読み込みが速くなる");
    expect(withoutRecommendedMark("**Recommended: B** It loads faster.")).toBe("It loads faster.");
  });

  it("leaves a recommendation of the older form as it is", () => {
    expect(withoutRecommendedMark("Bを勧めます。速いためです。")).toBe("Bを勧めます。速いためです。");
  });
});

describe("the questions that only confirm the recommended answer", () => {
  /** round builds a question comment of the questions qs, a [number, title] each, with the heading before the n-th. */
  const round = (qs: [number, string][], heading: string, before: number) =>
    "### 計画についての質問\n\n**計画：** 流れを分ける\n\n" +
    qs
      .map(([n, title], i) => (i === before ? `${heading}\n\n` : "") + `❓ **Q${n}** - **${title}**\n\n${title}にしますか。\n\nA: する\nB: しない\n\n➡️ **推奨：A** 速い`)
      .join("\n\n---\n\n") +
    "\n\n推奨どおりでよければ `/ok`、変えたい問いがあれば `/ok q2はB` のように返答してください。\n";

  it("are the questions after the heading, which no question keeps", () => {
    const p = parsePlanQuestions(round([[5, "範囲"], [6, "名前"], [7, "ログ"], [8, "既定値"]], "#### 推奨でよいか確認してほしい問い", 2))!;
    expect(p.questions.map((q) => [q.n, !!q.check])).toEqual([[5, false], [6, false], [7, true], [8, true]]);
    expect(p.questions.map((q) => q.recommendation)).toEqual(Array(4).fill("**推奨：A** 速い"));
    expect(JSON.stringify(p.questions)).not.toMatch(/推奨でよいか確認してほしい問い/);
    expect(p.tail).toMatch(/^推奨どおりでよければ/);
  });

  it("are every question when the heading comes before the first, and the head does not keep it", () => {
    const p = parsePlanQuestions(round([[1, "ログ"], [2, "既定値"]], "#### 推奨でよいか確認してほしい問い", 0))!;
    expect(p.questions.map((q) => !!q.check)).toEqual([true, true]);
    expect(p.head).not.toMatch(/推奨でよいか確認してほしい問い/);
    expect(p.head).toMatch(/\*\*計画：\*\* 流れを分ける$/);
  });

  it("are read under the English heading too", () => {
    const p = parsePlanQuestions(round([[1, "Scope"], [2, "Logs"]], "#### Please check the recommended answers", 1))!;
    expect(p.questions.map((q) => !!q.check)).toEqual([false, true]);
  });

  it("are none in a comment without the heading", () => {
    const p = parsePlanQuestions(round([[1, "範囲"], [2, "ログ"]], "", -1))!;
    expect(p.questions.map((q) => !!q.check)).toEqual([false, false]);
  });

  it("are none under the older headings, which are no longer read", () => {
    for (const heading of ["#### 推奨どおりで進めるもの", "#### Going ahead as recommended"]) {
      const p = parsePlanQuestions(round([[1, "範囲"], [2, "ログ"]], heading, 1))!;
      expect(p.questions.map((q) => !!q.check)).toEqual([false, false]);
    }
  });
});
