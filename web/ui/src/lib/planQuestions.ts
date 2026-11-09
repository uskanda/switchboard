// The question comment of a plan run, read back so that each question can be answered by choosing. The frame of the
// comment is switchboard's (internal/agent/planrun.go): a heading line per question, its body, the recommendation
// after an arrow, and "---" between questions. What is inside a body or a recommendation is the run's own writing,
// so the options are read in the many shapes a run writes them, and a question whose options cannot be read is
// answered in words alone.

import type { Comment } from "@/lib/api";
import { defineMessages, type Lang } from "@/lib/i18n";

export type Option = { label: string; text: string };

/** Group is one set of options that a single choice answers, with the heading it had on its line, if any. */
export type Group = { heading?: string; options: Option[] };

/** Part is a piece of a question's body: text, a group of options, or the Mermaid diagram before the options. */
export type Part = { kind: "text"; text: string } | { kind: "group"; group: Group } | { kind: "diagram"; source: string };

/** Question is one question of the comment; check is set on one that only confirms the recommended answer. */
export type Question = { n: number; title: string; parts: Part[]; recommendation: string; check?: boolean };

/** PlanQuestions is a question comment: what comes before the first question, the questions, and what follows them. */
export type PlanQuestions = { head: string; questions: Question[]; tail: string };

/**
 * Answer is the choice made in each group of a question (null when none is), and the note written for it. noteOnly
 * is set while the question is answered in the note alone, with every choice cleared; before keeps the choices made
 * until then, to give back once it is cleared. An answer kept from before these fields existed has neither.
 */
export type Answer = { choices: (string | null)[]; note: string; noteOnly?: boolean; before?: (string | null)[] };

const HEADING = /^❓️?\s*\*\*Q(\d+)\*\*\s*[-–—]\s*\*\*(.+?)\*\*\s*(?:[:：]\s*(.*))?$/u;
const ARROW = /^➡️?\s*/u;
const SEPARATOR = /^\s*(?:-{3,}|\*{3,}|_{3,})\s*$/;
// The heading before the questions that only confirm the recommended answer, in each language switchboard writes its
// comments in (internal/agent/messages.go); the language of the screen is another matter.
const CHECKS = /^#{1,6}\s+(?:推奨でよいか確認してほしい問い|Please check the recommended answers)\s*$/u;
const FENCE = /^\s*(?:```|~~~)/;
// The opening line of the code block of a question's diagram (internal/agent/planrun.go writes it).
const MERMAID = /^\s*```\s*mermaid\s*$/;
// An option on a line of its own: a list mark, bold or a parenthesis may come around the label, and a delimiter or
// a space after it.
const OPTION_LINE = /^\s*(?:[-*+]\s+)?(?:\*\*)?[(（]?([A-Z]|\d{1,2})[)）]?(?:\*\*)?(?:[:：.、．](?:\*\*)?\s*|\s+)(\S.*)$/u;
// One option of a line split by slashes.
const OPTION_SEGMENT = /^[(（]?([A-Z]|\d{1,2})[)）]?(?:[:：.、．]\s*|\s+)(\S.*)$/u;
const SLASH = /\s*[/／]\s*/u;
const INLINE_HEADING = /^([^:：/／]{1,20})[:：]\s*(.+)$/u;
const COLON_END = /^(.{1,20})[:：]\s*$/u;

const isDigits = (s: string) => /^\d+$/.test(s);

/** follows reports whether label b comes right after label a: B after A, 3 after 2. */
function follows(a: string, b: string): boolean {
  if (isDigits(a) !== isDigits(b)) return false;
  if (isDigits(a)) return Number(b) === Number(a) + 1;
  return b.charCodeAt(0) === a.charCodeAt(0) + 1;
}

const counts = (labels: string[]) => labels.length >= 2 && labels.every((l, i) => i === 0 || follows(labels[i - 1], l));

/** inlineGroup reads a line such as "範囲: A 全体 / B 購読フィード" as one group, or returns null. */
function inlineGroup(line: string): Group | null {
  const read = (rest: string): Option[] | null => {
    const segments = rest.trim().split(SLASH);
    if (segments.length < 2) return null;
    const options: Option[] = [];
    for (const s of segments) {
      const m = OPTION_SEGMENT.exec(s.trim());
      if (!m) return null;
      options.push({ label: m[1], text: m[2].trim() });
    }
    return counts(options.map((o) => o.label)) ? options : null;
  };
  const h = INLINE_HEADING.exec(line.trim());
  if (h) {
    const options = read(h[2]);
    if (options) return { heading: h[1].trim(), options };
  }
  const options = read(line);
  return options ? { options } : null;
}

/** parseParts splits the body of a question into its text and its groups of options. */
function parseParts(lines: string[]): Part[] {
  const parts: Part[] = [];
  let text: string[] = [];
  let run: (Option & { raw: string })[] = []; // the options read so far, with their lines in case they make no group
  let diagram: string[] | null = null; // the lines of a mermaid block being read
  let drawn = false; // a diagram or a group was read: a later mermaid block is text
  let blanks: string[] = []; // blank lines seen inside a run: dropped if the run goes on, text otherwise

  const pushText = () => {
    const t = text.join("\n").trim();
    if (t) parts.push({ kind: "text", text: t });
    text = [];
  };
  const pushGroup = (group: Group) => {
    // A short line ending in a colon right above the options names them.
    const last = text.at(-1);
    const named = !group.heading && last !== undefined ? COLON_END.exec(last.trim()) : null;
    if (named) {
      text.pop();
      group = { heading: named[1].trim(), options: group.options };
    }
    pushText();
    parts.push({ kind: "group", group });
    drawn = true;
  };
  const flush = () => {
    if (run.length >= 2) pushGroup({ options: run.map(({ label, text }) => ({ label, text })) });
    else text.push(...run.map((o) => o.raw));
    run = [];
    text.push(...blanks);
    blanks = [];
  };

  let fenced = false;
  for (const line of lines) {
    if (diagram) {
      if (!FENCE.test(line)) {
        diagram.push(line);
        continue;
      }
      // The first mermaid block before the options is the question's diagram.
      pushText();
      parts.push({ kind: "diagram", source: diagram.join("\n").trim() });
      diagram = null;
      drawn = true;
      continue;
    }
    if (!fenced && !drawn && run.length < 2 && MERMAID.test(line)) {
      flush();
      diagram = [];
      continue;
    }
    if (FENCE.test(line)) fenced = !fenced;
    if (fenced || FENCE.test(line)) {
      flush();
      text.push(line);
      continue;
    }
    if (line.trim() === "") {
      if (run.length) blanks.push(line);
      else text.push(line);
      continue;
    }
    const inline = inlineGroup(line);
    if (inline) {
      flush();
      pushGroup(inline);
      continue;
    }
    const m = OPTION_LINE.exec(line);
    const prev = run.at(-1);
    if (m && (!prev || follows(prev.label, m[1]))) {
      if (!prev) flush();
      blanks = [];
      run.push({ label: m[1], text: m[2].trim(), raw: line });
      continue;
    }
    if (prev && !blanks.length && /^\s{2,}\S/.test(line)) {
      // An indented line goes on with the option above it.
      prev.text = `${prev.text} ${line.trim()}`;
      prev.raw = `${prev.raw}\n${line}`;
      continue;
    }
    flush();
    if (m) {
      run.push({ label: m[1], text: m[2].trim(), raw: line });
      continue;
    }
    text.push(line);
  }
  // A mermaid block left open is text, as it was written.
  if (diagram) text.push("```mermaid", ...diagram);
  flush();
  pushText();
  return parts;
}

/**
 * parsePlanQuestions reads a question comment of a plan run. It returns null when the comment has no question
 * heading, so that the comment is drawn as any other. The questions after the heading of those that only confirm the
 * recommended answer are marked check; the heading itself is kept by no part of the result.
 */
export function parsePlanQuestions(body: string): PlanQuestions | null {
  const lines = body.replace(/\r\n?/g, "\n").split("\n");
  const starts = lines.flatMap((l, i) => (HEADING.test(l.trim()) ? [i] : []));
  if (!starts.length) return null;
  const checks = lines.findIndex((l) => CHECKS.test(l.trim()));
  const notChecks = (l: string) => !CHECKS.test(l.trim());

  const head = lines.slice(0, starts[0]).filter(notChecks).join("\n").trim();
  let tail = "";
  const questions = starts.map((start, k): Question => {
    const m = HEADING.exec(lines[start].trim())!;
    const last = k === starts.length - 1;
    const check = checks >= 0 && start > checks ? { check: true } : {};
    let block = lines.slice(start + 1, last ? lines.length : starts[k + 1]).filter(notChecks);
    if (m[3]) block = [m[3], ...block];
    // A non-last block ends at its separator; what follows the separator belongs to no question.
    const sep = last ? -1 : block.findIndex((l) => SEPARATOR.test(l));
    if (sep >= 0) block = block.slice(0, sep);

    const arrow = block.findIndex((l) => ARROW.test(l.trim()));
    if (arrow < 0) return { n: Number(m[1]), title: m[2].trim(), parts: parseParts(block), recommendation: "", ...check };

    let rec = [block[arrow].trim().replace(ARROW, ""), ...block.slice(arrow + 1)];
    if (last) {
      // The last recommendation ends with its paragraph: the hint on how to answer and the refused operations follow.
      const end = rec.findIndex((l) => l.trim() === "");
      if (end >= 0) {
        tail = rec.slice(end).join("\n").trim();
        rec = rec.slice(0, end);
      }
    }
    return { n: Number(m[1]), title: m[2].trim(), parts: parseParts(block.slice(0, arrow)), recommendation: rec.join("\n").trim(), ...check };
  });
  return { head, questions, tail };
}

/** groupsOf lists the groups of options of a question, in order. */
export const groupsOf = (q: Question): Group[] => q.parts.flatMap((p) => (p.kind === "group" ? [p.group] : []));

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * defaultAnswers chooses, in each group, the label of that group that comes first in the recommendation. A letter
 * or digit inside a word or a number is not a label. A group whose labels the recommendation never names starts
 * with no choice.
 */
export function defaultAnswers(questions: Question[]): Answer[] {
  return questions.map((q) => ({
    choices: groupsOf(q).map((g) => {
      let best: { at: number; label: string } | null = null;
      for (const { label } of g.options) {
        const at = q.recommendation.search(new RegExp(`(?<![A-Za-z0-9])${escape(label)}(?![A-Za-z0-9])`));
        if (at >= 0 && (!best || at < best.at)) best = { at, label };
      }
      return best?.label ?? null;
    }),
    note: "",
  }));
}

const oneLine = (s: string) => s.trim().replace(/\s*\n\s*/g, " ");

/** replyWords is the wording of a reply to a question comment in each language it is posted in. */
const replyWords = defineMessages({
  ja: {
    /** named is the choice of a group that has a heading, among two or more groups. */
    named: (heading: string, label: string) => `${heading}は${label}`,
    join: "、",
    noted: (choice: string, note: string) => `${choice}。補足：${note}`,
    none: "回答なし",
    overall: "全体への補足：",
  },
  en: {
    named: (heading, label) => `${heading}: ${label}`,
    join: "; ",
    noted: (choice, note) => `${choice}. Note: ${note}`,
    none: "No answer",
    overall: "Overall note:",
  },
});

/**
 * composeReply writes the reply to a question comment in the language lang: a line per question with its number,
 * title and answer, then the text of the reply field as a note on the whole. It never starts with the approval word,
 * so it can never be taken for the approval of an earlier plan.
 */
export function composeReply(lang: Lang, questions: Question[], answers: Answer[], overall: string): string {
  const w = replyWords[lang];
  const lines = questions.map((q, i) => {
    const a = answers[i] ?? { choices: [], note: "" };
    const groups = groupsOf(q);
    const chosen = groups.flatMap((g, j) => {
      const label = a.choices[j];
      if (!label) return [];
      return [groups.length > 1 && g.heading ? w.named(g.heading, label) : label];
    });
    const choice = chosen.join(w.join);
    const note = oneLine(a.note);
    const answer = choice && note ? w.noted(choice, note) : choice || note || w.none;
    return `Q${q.n} ${q.title}: ${answer}`;
  });
  const whole = overall.trim();
  return whole ? `${lines.join("\n")}\n\n${w.overall}\n${whole}` : lines.join("\n");
}

// The agent's hint on answering with the approval word, in each of its languages (internal/agent/messages.go). The
// web inbox answers by choosing, so the hint is left out where the questions are drawn for answering.
const ANSWER_HINTS = [/^推奨どおりでよければ /, /^Reply `.*` to accept the recommended answers/];

/** withoutAnswerHint drops the paragraphs of a comment's tail that are the hint on answering with the approval word. */
export const withoutAnswerHint = (tail: string) =>
  tail
    .split(/\n\s*\n/)
    .filter((p) => !ANSWER_HINTS.some((h) => h.test(p.trim())))
    .join("\n\n")
    .trim();

// The paragraph that carries the plan's title right after the heading of a plan-stage comment, in each language the
// agent writes (internal/agent/planrun.go, planTitleLine).
const TITLE_LINE = /^\*\*(?:計画：|Plan:)\*\*/u;

/** Head is the start of a question comment split for drawing: what is shown, and what the agent looked at, folded. */
export type Head = { shown: string; folded: string };

/**
 * splitHead splits the start of a question comment after the paragraph with the plan's title: the heading and the
 * title are shown, and the paragraphs after them, what the agent looked at, are folded. Without a title line it
 * cannot tell where they start, so nothing is folded.
 */
export function splitHead(head: string): Head {
  const paragraphs = head.split(/\n\s*\n/);
  const at = paragraphs.findIndex((p) => TITLE_LINE.test(p.trim()));
  if (at < 0) return { shown: head, folded: "" };
  return { shown: paragraphs.slice(0, at + 1).join("\n\n").trim(), folded: paragraphs.slice(at + 1).join("\n\n").trim() };
}

/**
 * setNoteOnly ticks (on) or clears the box of answering a question in the note alone. Ticking it clears every choice
 * and keeps them in before; clearing it gives them back.
 */
export function setNoteOnly(a: Answer, on: boolean): Answer {
  if (on) return { ...a, noteOnly: true, before: a.choices, choices: a.choices.map(() => null) };
  return { choices: a.before ?? a.choices, note: a.note, noteOnly: false };
}

/** QuestionState is where the answer to a question stands, as its chip says. */
export type QuestionState = "noteOnly" | "unanswered" | "changed" | "recommended" | "chosen";

/**
 * questionState says where the answer to q stands: answered in the note alone, with a note written; unanswered, with a
 * group left without a choice (or no groups) and no note, the box of the note alone included, as it sends 回答なし; changed, when a group the recommendation names has another option or none;
 * recommended, when every such group keeps it; and chosen, when the recommendation names no group. noted says a note
 * comes on top of the choices.
 */
export function questionState(q: Question, a: Answer): { state: QuestionState; noted: boolean } {
  const groups = groupsOf(q);
  const recommended = defaultAnswers([q])[0].choices;
  const note = a.note.trim() !== "";
  const choice = (j: number) => a.choices[j] ?? null;
  if (note && (a.noteOnly || groups.length === 0)) return { state: "noteOnly", noted: false };
  if (!note && (groups.length === 0 || groups.some((_, j) => choice(j) === null))) return { state: "unanswered", noted: false };
  const named = groups.map((_, j) => j).filter((j) => recommended[j] !== null);
  if (named.some((j) => choice(j) !== recommended[j])) return { state: "changed", noted: note };
  return { state: named.length > 0 ? "recommended" : "chosen", noted: note };
}

/**
 * Decided is one item of what earlier rounds settled: a question with the answer its reply gave, or, for a round
 * answered in words, the first line of that reply with the numbers of the round's questions.
 */
export type Decided = { kind: "question"; n: number; title: string; answer: string } | { kind: "reply"; from: number; to: number; text: string };

const decidedWords = defineMessages({
  ja: { recommended: "推奨どおり" },
  en: { recommended: "As recommended" },
});

const LEADING_LABEL = /^([A-Z]|\d{1,2})(?=$|[。．.、,;:：\s])/u;

/** withOption adds the text of the option an answer starts with, when the question has a single group with it. */
function withOption(q: Question, answer: string): string {
  const groups = groupsOf(q);
  const m = LEADING_LABEL.exec(answer);
  const option = m && groups.length === 1 ? groups[0].options.find((o) => o.label === m[1]) : undefined;
  return option ? `${option.label} ${option.text}${answer.slice(option.label.length)}` : answer;
}

/** recommendedAnswer is the answer the approval word alone gives q: its recommended options, as a reply writes them. */
function recommendedAnswer(lang: Lang, q: Question): string {
  const w = replyWords[lang];
  const groups = groupsOf(q);
  const labels = defaultAnswers([q])[0].choices;
  const chosen = groups.flatMap((g, j) => {
    const label = labels[j];
    if (!label) return [];
    return [groups.length > 1 && g.heading ? w.named(g.heading, label) : label];
  });
  return chosen.length > 0 ? withOption(q, chosen.join(w.join)) : decidedWords[lang].recommended;
}

/** answerLine finds the answer to q among the lines of a reply: the last line that starts with its number and title. */
function answerLine(q: Question, lines: string[]): string | null {
  const exact = `Q${q.n} ${q.title}: `;
  let found: string | null = null;
  for (const line of lines) {
    const l = line.trim();
    if (l.startsWith(exact)) found = l.slice(exact.length).trim();
    else if (l.startsWith(`Q${q.n} `)) {
      const at = l.indexOf(": ");
      if (at >= 0) found = l.slice(at + 2).trim();
    }
  }
  return found;
}

/**
 * decidedBefore lists what the rounds of the current cycle before the question comment at latest settled. A cycle
 * starts at a question comment whose first question is Q1, as the numbers start over with every plan. A round's
 * reply is what people wrote after its question comment, up to the agent's next post: its lines of the form
 * "Q<n> <title>: <answer>", the approval word alone (the recommended options), or else words, of which the first
 * line stands for the round. A question its reply has no line for is left out.
 */
export function decidedBefore(lang: Lang, comments: Comment[], latest: number, word: string): Decided[] {
  const rounds = comments.flatMap((c, i) => {
    if (i > latest || !c.agent || c.kind !== "waiting") return [];
    const parsed = parsePlanQuestions(c.body);
    return parsed && parsed.questions.length > 0 ? [{ i, questions: parsed.questions }] : [];
  });
  const start = rounds.reduce((at, r, k) => (r.questions[0].n === 1 ? k : at), 0);
  const earlier = rounds.slice(start).filter((r) => r.i < latest);
  return earlier.flatMap(({ i, questions }) => {
    const replies: string[] = [];
    for (let k = i + 1; k < comments.length && !comments[k].agent; k++) replies.push(comments[k].body);
    const lines = replies.flatMap((r) => r.replace(/\r\n?/g, "\n").split("\n"));
    const answered = questions.flatMap((q): Decided[] => {
      const answer = answerLine(q, lines);
      return answer === null ? [] : [{ kind: "question", n: q.n, title: q.title, answer: withOption(q, answer) }];
    });
    if (answered.length > 0) return answered;
    if (replies.some((r) => r.trim() === word)) {
      return questions.map((q): Decided => ({ kind: "question", n: q.n, title: q.title, answer: recommendedAnswer(lang, q) }));
    }
    const first = replies.map((r) => r.trim()).find((r) => r !== "");
    if (first === undefined) return [];
    return [{ kind: "reply", from: questions[0].n, to: questions[questions.length - 1].n, text: first.split("\n")[0].trim() }];
  });
}

// The bold start of a recommendation that switchboard writes before the reason, in each of its languages
// (internal/agent/messages.go, planRecommended).
const RECOMMENDED_MARK = /^\*\*(?:推奨：|Recommended: )(?:[A-Z]|\d{1,2})\*\*\s*/u;

/** withoutRecommendedMark drops the bold label a recommendation starts with, which the option it sits under shows. */
export const withoutRecommendedMark = (recommendation: string) => recommendation.replace(RECOMMENDED_MARK, "");
