import type { ReactNode } from "react";
import { Markdown } from "@/components/Markdown";
import { Textarea } from "@/components/ui/textarea";
import { withoutAnswerHint, type Answer, type Group, type PlanQuestions as Parsed, type Question } from "@/lib/planQuestions";

const NONE = "選ばない（補足で答える）";

function Choices({ q, j, group, choice, disabled, onChoose }: {
  q: Question; j: number; group: Group; choice: string | null; disabled: boolean; onChoose: (label: string | null) => void;
}) {
  const name = `q${q.n}-g${j}`;
  const row = "flex min-h-[44px] cursor-pointer items-start gap-sm py-xs";
  const radio = "mt-[0.3em] size-4 shrink-0 accent-primary";
  return (
    <fieldset className="flex flex-col">
      <legend className="text-label-md font-medium text-muted">{group.heading ?? `Q${q.n}の選択肢`}</legend>
      {group.options.map((o) => (
        <label key={o.label} className={row}>
          <input type="radio" name={name} value={o.label} checked={choice === o.label} disabled={disabled} onChange={() => onChoose(o.label)} className={radio} />
          <span className="min-w-0">
            <span className="font-medium">{o.label}</span> <Markdown text={o.text} inline />
          </span>
        </label>
      ))}
      <label className={`${row} text-muted`}>
        <input type="radio" name={name} value="" checked={choice === null} disabled={disabled} onChange={() => onChoose(null)} className={radio} />
        <span>{NONE}</span>
      </label>
    </fieldset>
  );
}

function OneQuestion({ q, answer, disabled, onChange }: { q: Question; answer: Answer; disabled: boolean; onChange: (a: Answer) => void }) {
  const id = `q${q.n}-note`;
  const lastGroup = q.parts.reduce((at, p, i) => (p.kind === "group" ? i : at), -1);
  const note = (
    <div key="note" className="flex flex-col gap-xs">
      <label htmlFor={id} className="text-label-md font-medium text-muted">
        Q{q.n}の補足
      </label>
      <Textarea id={id} value={answer.note} disabled={disabled} onChange={(e) => onChange({ ...answer, note: e.target.value })} className="min-h-20" />
    </div>
  );
  const body: ReactNode[] = [];
  let k = 0; // the number of the group among the question's groups
  q.parts.forEach((p, i) => {
    if (p.kind === "text") {
      body.push(<Markdown key={i} text={p.text} />);
      return;
    }
    const g = k++;
    const choose = (label: string | null) => onChange({ ...answer, choices: answer.choices.map((c, m) => (m === g ? label : c)) });
    body.push(<Choices key={i} q={q} j={g} group={p.group} choice={answer.choices[g] ?? null} disabled={disabled} onChoose={choose} />);
    // The free text goes right under the last group of options.
    if (i === lastGroup) body.push(note);
  });
  return (
    <section aria-labelledby={`q${q.n}-title`} className="flex flex-col gap-sm">
      <h4 id={`q${q.n}-title`} className="font-semibold break-words">
        Q{q.n} {q.title}
      </h4>
      {body}
      {lastGroup < 0 && note}
      {q.recommendation && (
        <div className="flex flex-col gap-xs">
          <span className="text-label-md font-medium text-muted">推奨</span>
          <Markdown text={q.recommendation} />
        </div>
      )}
    </section>
  );
}

/**
 * PlanQuestions draws the latest question comment of a plan for answering: each question with its groups of options
 * as radio buttons and a field for a note. What is chosen lives with the reply (IssuePane), which writes it out.
 */
export function PlanQuestions({ parsed, answers, disabled, onChange }: {
  parsed: Parsed; answers: Answer[]; disabled: boolean; onChange: (answers: Answer[]) => void;
}) {
  const tail = withoutAnswerHint(parsed.tail);
  return (
    <div className="flex flex-col gap-lg">
      {parsed.head && <Markdown text={parsed.head} />}
      {parsed.questions.map((q, i) => (
        <OneQuestion key={q.n} q={q} answer={answers[i]} disabled={disabled} onChange={(a) => onChange(answers.map((b, k) => (k === i ? a : b)))} />
      ))}
      {tail && <Markdown text={tail} />}
    </div>
  );
}
