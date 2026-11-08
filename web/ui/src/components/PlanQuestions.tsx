import { useState, type ReactNode } from "react";
import { Markdown } from "@/components/Markdown";
import { Textarea } from "@/components/ui/textarea";
import { defineMessages, useMessages } from "@/lib/i18n";
import {
  defaultAnswers,
  questionState,
  setNoteOnly,
  splitHead,
  withoutAnswerHint,
  type Answer,
  type Decided,
  type Group,
  type PlanQuestions as Parsed,
  type Question,
  type QuestionState,
} from "@/lib/planQuestions";
import { cn } from "@/lib/utils";

const messages = defineMessages({
  ja: {
    options: (n: number) => `Q${n}の選択肢`,
    /** recommendedMark follows the option the recommendation names. */
    recommendedMark: "（推奨）",
    note: (n: number) => `Q${n}の補足`,
    addNote: (n: number) => `Q${n}に別の答えや補足を書く`,
    noteOnly: "選択肢を選ばず、補足だけで答える",
    /** noteOnlyLabel names the box of one question, for a screen reader: it holds the words shown. */
    noteOnlyLabel: (n: number) => `Q${n}は選択肢を選ばず、補足だけで答える`,
    recommended: "推奨",
    looked: "エージェントが調べたこと",
    settled: "これまでに決まったこと",
    /** colon goes between a question and its answer in what is settled. */
    colon: "：",
    range: (from: number, to: number) => (from === to ? `Q${from}` : `Q${from}〜Q${to}`),
    states: { noteOnly: "補足だけ", unanswered: "未回答", changed: "推奨と違う", recommended: "推奨どおり", chosen: "選択済み" } as Record<QuestionState, string>,
    noted: "＋補足",
    chips: "問いの状態",
  },
  en: {
    options: (n) => `Q${n} options`,
    recommendedMark: " (recommended)",
    note: (n) => `Q${n} note`,
    addNote: (n) => `Write another answer or a note for Q${n}`,
    noteOnly: "Answer in the note alone, without an option",
    noteOnlyLabel: (n) => `Q${n}: answer in the note alone, without an option`,
    recommended: "Recommended",
    looked: "What the agent looked at",
    settled: "Settled so far",
    colon: ": ",
    range: (from, to) => (from === to ? `Q${from}` : `Q${from}–Q${to}`),
    states: { noteOnly: "Note only", unanswered: "Unanswered", changed: "Changed", recommended: "As recommended", chosen: "Chosen" },
    noted: " + note",
    chips: "Question status",
  },
});

/** Ids of a question's card and its title, which the chips scroll to and focus. */
const cardId = (n: number) => `q${n}-card`;
const titleId = (n: number) => `q${n}-title`;

/**
 * Choices is one group of options as radio buttons; the option the recommendation names is marked in words. When
 * reason is given, the recommendation's text is drawn right under that option, lined up with the options' words, and
 * stays there whichever option is chosen.
 */
function Choices({ q, j, group, choice, recommended, reason, disabled, onChoose }: {
  q: Question; j: number; group: Group; choice: string | null; recommended: string | null; reason?: string; disabled: boolean;
  onChoose: (label: string) => void;
}) {
  const t = useMessages(messages);
  const name = `q${q.n}-g${j}`;
  const reasonId = `${name}-reason`;
  return (
    <fieldset className="flex flex-col">
      <legend className="text-label-md font-medium text-muted">{group.heading ?? t.options(q.n)}</legend>
      {group.options.map((o) => (
        <div key={o.label} className="flex flex-col">
          <label className="flex min-h-[44px] cursor-pointer items-start gap-sm py-xs">
            <input
              type="radio"
              name={name}
              value={o.label}
              checked={choice === o.label}
              disabled={disabled}
              onChange={() => onChoose(o.label)}
              aria-describedby={reason && o.label === recommended ? reasonId : undefined}
              className="mt-[0.3em] size-4 shrink-0 accent-primary"
            />
            <span className="min-w-0">
              <span className="font-medium">{o.label}</span> <Markdown text={o.text} inline />
              {o.label === recommended && <span className="text-label-md font-medium text-muted">{t.recommendedMark}</span>}
            </span>
          </label>
          {reason && o.label === recommended && (
            <div id={reasonId} className="pb-sm pl-[calc(1rem+var(--spacing-sm))] text-muted">
              <Markdown text={reason} />
            </div>
          )}
        </div>
      ))}
    </fieldset>
  );
}

/**
 * OneQuestion is one question in a card of its own: its number, its place in the round and its title, then its body
 * with its groups of options, the recommendation and the note. The recommendation goes right under the option it
 * names when the question has a single group that the recommendation names; otherwise under the last group, or under
 * the body without groups. The note's field is folded behind a button while every group has an option chosen and
 * nothing is written in it, so a long round keeps the reply's buttons near on a phone (issue #180). It is open for a
 * question without options, for one with a group left unchosen, and once something is written in it; pressing the
 * button opens it with the focus in it. With the field open, a box answers the question in the note alone.
 */
function OneQuestion({ q, place, total, answer, disabled, onChange }: {
  q: Question; place: number; total: number; answer: Answer; disabled: boolean; onChange: (a: Answer) => void;
}) {
  const t = useMessages(messages);
  const id = `q${q.n}-note`;
  const lastGroup = q.parts.reduce((at, p, i) => (p.kind === "group" ? i : at), -1);
  const groups = q.parts.filter((p) => p.kind === "group").length;
  const recommended = defaultAnswers([q])[0].choices;
  // The recommendation sits under its option only when there is a single group and it names one of its options.
  const underOption = groups === 1 && recommended[0] !== null && q.recommendation !== "";
  const [pressed, setPressed] = useState(false);
  const noteOnly = !!answer.noteOnly;
  const open = pressed || noteOnly || lastGroup < 0 || answer.note !== "" || answer.choices.some((c) => c === null);
  const reason = q.recommendation && !underOption && (
    <div key="reason" className="flex flex-col gap-xs">
      <span className="text-label-md font-medium text-muted">{t.recommended}</span>
      <Markdown text={q.recommendation} />
    </div>
  );
  const note = open ? (
    <div key="note" className="flex flex-col gap-xs">
      <label htmlFor={id} className="text-label-md font-medium text-muted">
        {t.note(q.n)}
      </label>
      <Textarea
        id={id}
        value={answer.note}
        disabled={disabled}
        autoFocus={pressed}
        onChange={(e) => onChange({ ...answer, note: e.target.value })}
        className="min-h-20"
      />
      {groups > 0 && (
        <label className="flex min-h-11 cursor-pointer items-center gap-sm text-label-md">
          <input
            type="checkbox"
            checked={noteOnly}
            disabled={disabled}
            onChange={(e) => onChange(setNoteOnly(answer, e.target.checked))}
            aria-label={t.noteOnlyLabel(q.n)}
            className="size-4 shrink-0 accent-primary"
          />
          <span>{t.noteOnly}</span>
        </label>
      )}
    </div>
  ) : (
    <button
      key="note"
      type="button"
      disabled={disabled}
      onClick={() => setPressed(true)}
      className="inline-flex min-h-11 items-center self-start text-label-md text-muted underline focus-visible:outline-2 focus-visible:outline-primary"
    >
      {t.addNote(q.n)}
    </button>
  );
  const body: ReactNode[] = [];
  let k = 0; // the number of the group among the question's groups
  q.parts.forEach((p, i) => {
    if (p.kind === "text") {
      body.push(<Markdown key={i} text={p.text} />);
      return;
    }
    const g = k++;
    const choose = (label: string) => onChange({ ...answer, choices: answer.choices.map((c, m) => (m === g ? label : c)) });
    body.push(
      <Choices
        key={i}
        q={q}
        j={g}
        group={p.group}
        choice={answer.choices[g] ?? null}
        recommended={recommended[g] ?? null}
        reason={underOption ? q.recommendation : undefined}
        disabled={disabled || noteOnly}
        onChoose={choose}
      />,
    );
    // The recommendation, when not under its option, and the free text go right under the last group of options.
    if (i === lastGroup) body.push(reason, note);
  });
  return (
    <section
      id={cardId(q.n)}
      aria-labelledby={titleId(q.n)}
      className="flex scroll-mt-[calc(var(--header-h,0px)+var(--pane-head-h,0px))] flex-col gap-sm rounded-md border border-outline p-md lg:scroll-mt-[var(--pane-head-h,0px)]"
    >
      <h3 id={titleId(q.n)} tabIndex={-1} className="font-semibold break-words outline-none focus-visible:outline-2 focus-visible:outline-primary">
        <span className="font-mono text-mono-sm">Q{q.n}</span>{" "}
        <span aria-hidden="true" className="font-mono text-mono-sm font-normal text-muted">
          {place}/{total}
        </span>{" "}
        {q.title}
      </h3>
      {body}
      {lastGroup < 0 && [reason, note]}
    </section>
  );
}

/** Settled lists what the earlier rounds of the plan settled, a line a question, each cut at two lines. */
function Settled({ decided }: { decided: Decided[] }) {
  const t = useMessages(messages);
  return (
    <section aria-labelledby="settled-title" className="flex flex-col gap-xs">
      <h3 id="settled-title" className="text-label-md font-medium text-muted">
        {t.settled}
      </h3>
      <ul className="flex flex-col gap-xs">
        {decided.map((d) => (
          <li key={d.kind === "question" ? `q${d.n}` : `r${d.from}`} className="line-clamp-2 break-words">
            <span className="font-semibold">{d.kind === "question" ? `Q${d.n} ${d.title}` : t.range(d.from, d.to)}</span>
            {t.colon}
            <span>
              <Markdown text={d.kind === "question" ? d.answer : d.text} inline />
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Head draws the start of the question comment: the heading and the plan's title, and what the agent looked at, folded. */
function Head({ head }: { head: string }) {
  const t = useMessages(messages);
  const { shown, folded } = splitHead(head);
  return (
    <>
      {shown && <Markdown text={shown} />}
      {folded && (
        <details className="group">
          <summary className="inline-flex min-h-11 cursor-pointer list-none items-center gap-xs rounded-sm text-label-md text-muted focus-visible:outline-2 focus-visible:outline-primary [&::-webkit-details-marker]:hidden">
            <span aria-hidden="true" className="transition-transform group-open:rotate-90">
              ▸
            </span>
            {t.looked}
          </summary>
          <div className="pt-xs">
            <Markdown text={folded} />
          </div>
        </details>
      )}
    </>
  );
}

/**
 * PlanQuestions draws the latest question comment of a plan for answering: what earlier rounds settled, the start of
 * the comment, then each question in a card with its groups of options as radio buttons and a field for a note. What
 * is chosen lives with the reply (IssuePane), which writes it out.
 */
export function PlanQuestions({ parsed, answers, decided = [], disabled, onChange }: {
  parsed: Parsed; answers: Answer[]; decided?: Decided[]; disabled: boolean; onChange: (answers: Answer[]) => void;
}) {
  const tail = withoutAnswerHint(parsed.tail);
  const total = parsed.questions.length;
  return (
    <div className="flex flex-col gap-lg">
      {decided.length > 0 && <Settled decided={decided} />}
      {parsed.head && <Head head={parsed.head} />}
      {parsed.questions.map((q, i) => (
        <OneQuestion
          key={q.n}
          q={q}
          place={i + 1}
          total={total}
          answer={answers[i]}
          disabled={disabled}
          onChange={(a) => onChange(answers.map((b, k) => (k === i ? a : b)))}
        />
      ))}
      {tail && <Markdown text={tail} />}
    </div>
  );
}

/**
 * QuestionChips lists where the answer to every question stands, one button a question, above the reply that is sent.
 * Pressing one scrolls its question's card to just below the pinned head and puts the focus on its title. Only an
 * unanswered question is in color, as it is the one to look at before sending.
 */
export function QuestionChips({ questions, answers }: { questions: Question[]; answers: Answer[] }) {
  const t = useMessages(messages);
  const go = (n: number) => {
    document.getElementById(cardId(n))?.scrollIntoView({ block: "start" });
    document.getElementById(titleId(n))?.focus({ preventScroll: true });
  };
  return (
    <ul aria-label={t.chips} className="flex flex-wrap gap-xs">
      {questions.map((q, i) => {
        const { state, noted } = questionState(q, answers[i] ?? { choices: [], note: "" });
        return (
          <li key={q.n}>
            <button
              type="button"
              onClick={() => go(q.n)}
              className={cn(
                "inline-flex min-h-11 items-center gap-xs rounded-md border border-outline bg-surface-raised px-sm text-label-md focus-visible:outline-2 focus-visible:outline-primary",
                state === "unanswered" ? "font-medium text-attention" : "text-on-surface",
              )}
            >
              {`Q${q.n} ${t.states[state]}${noted ? t.noted : ""}`}
            </button>
          </li>
        );
      })}
    </ul>
  );
}
