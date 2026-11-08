// The question comment of a plan run, read back so that each question can be answered by choosing. The frame of the
// comment is switchboard's (internal/agent/planrun.go): a heading line per question, its body, the recommendation
// after an arrow, and "---" between questions. What is inside a body or a recommendation is the run's own writing,
// so the options are read in the many shapes a run writes them, and a question whose options cannot be read is
// answered in words alone.

import { defineMessages, type Lang } from "@/lib/i18n";

export type Option = { label: string; text: string };

/** Group is one set of options that a single choice answers, with the heading it had on its line, if any. */
export type Group = { heading?: string; options: Option[] };

export type Part = { kind: "text"; text: string } | { kind: "group"; group: Group };

export type Question = { n: number; title: string; parts: Part[]; recommendation: string };

/** PlanQuestions is a question comment: what comes before the first question, the questions, and what follows them. */
export type PlanQuestions = { head: string; questions: Question[]; tail: string };

/** Answer is the choice made in each group of a question (null when none is), and the note written for it. */
export type Answer = { choices: (string | null)[]; note: string };

const HEADING = /^❓️?\s*\*\*Q(\d+)\*\*\s*[-–—]\s*\*\*(.+?)\*\*\s*(?:[:：]\s*(.*))?$/u;
const ARROW = /^➡️?\s*/u;
const SEPARATOR = /^\s*(?:-{3,}|\*{3,}|_{3,})\s*$/;
const FENCE = /^\s*(?:```|~~~)/;
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
  flush();
  pushText();
  return parts;
}

/**
 * parsePlanQuestions reads a question comment of a plan run. It returns null when the comment has no question
 * heading, so that the comment is drawn as any other.
 */
export function parsePlanQuestions(body: string): PlanQuestions | null {
  const lines = body.replace(/\r\n?/g, "\n").split("\n");
  const starts = lines.flatMap((l, i) => (HEADING.test(l.trim()) ? [i] : []));
  if (!starts.length) return null;

  const head = lines.slice(0, starts[0]).join("\n").trim();
  let tail = "";
  const questions = starts.map((start, k) => {
    const m = HEADING.exec(lines[start].trim())!;
    const last = k === starts.length - 1;
    let block = lines.slice(start + 1, last ? lines.length : starts[k + 1]);
    if (m[3]) block = [m[3], ...block];
    // A non-last block ends at its separator; what follows the separator belongs to no question.
    const sep = last ? -1 : block.findIndex((l) => SEPARATOR.test(l));
    if (sep >= 0) block = block.slice(0, sep);

    const arrow = block.findIndex((l) => ARROW.test(l.trim()));
    if (arrow < 0) return { n: Number(m[1]), title: m[2].trim(), parts: parseParts(block), recommendation: "" };

    let rec = [block[arrow].trim().replace(ARROW, ""), ...block.slice(arrow + 1)];
    if (last) {
      // The last recommendation ends with its paragraph: the hint on how to answer and the refused operations follow.
      const end = rec.findIndex((l) => l.trim() === "");
      if (end >= 0) {
        tail = rec.slice(end).join("\n").trim();
        rec = rec.slice(0, end);
      }
    }
    return { n: Number(m[1]), title: m[2].trim(), parts: parseParts(block.slice(0, arrow)), recommendation: rec.join("\n").trim() };
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
