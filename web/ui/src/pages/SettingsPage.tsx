import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { IssueRef } from "@/components/IssueRef";
import { Loading } from "@/components/Loading";
import { ProjectsSection } from "@/components/ProjectsSection";
import { Shell } from "@/components/Shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { useAuth } from "@/lib/auth";
import { ApiError, getInstructions, saveInstructions, type Instruction, type InstructionKind, type Instructions } from "@/lib/api";
import { defineMessages, errorText, useLanguageChoice, useMessages, type Lang, type LangChoice } from "@/lib/i18n";
import { common } from "@/lib/i18n/common";
import { instructionDraftOf, keepInstructionDraft } from "@/lib/instructionDrafts";
import { refText } from "@/lib/ref";
import { usePageTitle } from "@/lib/title";
import { toast } from "@/lib/toast";

/** messages is the text of the settings page: the display section, the instruction fields and their refusals. */
const messages = defineMessages({
  ja: {
    title: "設定",
    display: "表示",
    language: "言語",
    auto: "自動",
    autoNote: "自動は、ブラウザの言語に従います。",
    adminOnly: "プロジェクトと追加の指示を変えられるのは管理者だけです。",
    tooLarge: "32 KiBを超えているため、保存していません。",
    secret: "秘密情報らしいものを含むため、保存していません。",
    unknownProject: "設定に無いプロジェクトのため、保存していません。",
    forbidden: "設定を変えられるのは管理者だけです。",
    unreachable: common.ja.unreachable,
    failed: common.ja.failed,
    allProjects: "すべてのプロジェクト",
    /** saved names the field in the toast of a save: its project, or none for the field of all projects. */
    saved: (project: string | null, noun: string) => `${project ?? "すべてのプロジェクト"}の${noun}を保存しました`,
    unsaved: "未保存の変更があります",
    saving: "保存しています",
    save: "保存する",
    /** about is the note under a kind's heading: what runs get the kind, then how the fields join. */
    about: (about: string) => `${about}すべてのプロジェクトの指示のあとに、そのプロジェクトの指示をつなげます。`,
    host: (host: ReactNode, command: ReactNode) => (
      <>
        端末 {host} のファイルです。エージェントが別の端末で動くときは、その端末で {command} を使ってください。
      </>
    ),
    kinds: {
      plan: {
        heading: "計画の実行への追加の指示",
        noun: "計画の指示",
        about: "計画から始めたissueの、計画を立てる実行にだけ渡します。",
      },
      run: {
        heading: "実装の実行への追加の指示",
        noun: "実装の指示",
        about: "ファイルを書く実行（実装、承認のあとの実装、CIやpushの失敗とbaseとの衝突を直す実行）に渡します。",
      },
      implement: {
        heading: "実装の実行だけへの追加の指示",
        noun: "実装だけの指示",
        about:
          "直接の実装と承認のあとの実装にだけ、実装の実行への指示のあとに渡します。計画の実行と、直す実行には渡しません。スクリーンショットを撮る指示はここに書きます。",
      },
    },
  },
  en: {
    title: "Settings",
    display: "Display",
    language: "Language",
    auto: "Automatic",
    autoNote: "Automatic follows your browser's language.",
    adminOnly: "Only an admin can change the projects and additional instructions.",
    tooLarge: "Not saved: it is over 32 KiB.",
    secret: "Not saved: it seems to contain a secret.",
    unknownProject: "Not saved: the project is not in the config.",
    forbidden: "Only an admin can change the settings.",
    unreachable: common.en.unreachable,
    failed: common.en.failed,
    allProjects: "All projects",
    saved: (project, noun) => `Saved the ${noun} for ${project ?? "all projects"}`,
    unsaved: "Unsaved changes",
    saving: "Saving",
    save: "Save",
    about: (about) => `${about} A project's instructions follow those for all projects.`,
    host: (host, command) => (
      <>
        These files are on host {host}. If the agent runs on another host, use {command} there.
      </>
    ),
    kinds: {
      plan: {
        heading: "Additional instructions for plan runs",
        noun: "plan instructions",
        about: "For an issue that starts with a plan, only the run that makes the plan gets these.",
      },
      run: {
        heading: "Additional instructions for implementation runs",
        noun: "implementation instructions",
        about:
          "The runs that write files get these: implementation, implementation after approval, and the runs that fix a CI or push failure or a conflict with the base branch.",
      },
      implement: {
        heading: "Additional instructions for implementation runs only",
        noun: "implementation-only instructions",
        about:
          "Only direct implementation and implementation after approval get these, after the implementation instructions. Plan runs and fix runs do not. Write instructions to take screenshots here.",
      },
    },
  },
});

type Text = (typeof messages)["ja"];

/** failure is the words for an error of reading or saving the instructions, in the language shown. */
function failure(t: Text, err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "too_large":
        return t.tooLarge;
      case "secret":
        return t.secret;
      case "unknown_project":
        return t.unknownProject;
      case "forbidden":
        return t.forbidden;
      case "unavailable":
        return t.unreachable;
    }
  }
  return t.failed(errorText(err));
}

/** Failure holds an error to show, so its words follow a change of the language. */
type Failure = { cause: unknown };

/**
 * Field is one instruction file: its own text, its own save and its own refusal, so one refusal touches no other. A
 * save that worked is told in a toast that names the field.
 * While the text differs from the one last saved (at first, the one read), the field says it is not saved, and the
 * browser asks before the page unloads. The text not saved is kept in memory (lib/instructionDrafts.ts), so leaving
 * the settings and coming back finds it again.
 */
function Field({ item, kind }: { item: Instruction; kind: InstructionKind }) {
  const t = useMessages(messages);
  const [saved, setSaved] = useState(item.content);
  const [text, setText] = useState(() => instructionDraftOf(kind, item.target) ?? item.content);
  const [saving, setSaving] = useState(false);
  const dirty = text !== saved;
  useEffect(() => keepInstructionDraft(kind, item.target, text, saved), [kind, item.target, text, saved]);
  useEffect(() => {
    if (!dirty) return;
    const ask = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", ask);
    return () => window.removeEventListener("beforeunload", ask);
  }, [dirty]);
  const [error, setError] = useState<Failure | null>(null);
  const id = `instructions-${kind}-${item.target || "global"}`;
  const project = item.target ? refText(item.target) : null;

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await saveInstructions(item.target, kind, text);
      setSaved(text);
      toast.success(t.saved(project, t.kinds[kind].noun));
    } catch (err) {
      setError({ cause: err });
    } finally {
      setSaving(false);
    }
  }

  return (
    <section aria-labelledby={`${id}-label`}>
      <Card>
        <form onSubmit={submit} className="flex flex-col gap-sm">
          <label id={`${id}-label`} htmlFor={id} className={item.target ? undefined : "text-label-md font-medium"}>
            {item.target ? <IssueRef value={item.target} /> : t.allProjects}
          </label>
          <Textarea
            id={id}
            value={text}
            onChange={(e) => setText(e.target.value)}
            disabled={saving}
          />
          {error && <Alert>{failure(t, error.cause)}</Alert>}
          <div className="flex flex-wrap items-center justify-end gap-sm">
            {dirty && <span className="text-label-md text-muted">{t.unsaved}</span>}
            <Button type="submit" variant="secondary" disabled={saving}>
              {saving ? t.saving : t.save}
            </Button>
          </div>
        </form>
      </Card>
    </section>
  );
}

/** The command that edits the instructions of each kind on the host the agent runs on. */
const commands: Record<InstructionKind, string> = {
  plan: "switchboard instructions edit",
  run: "switchboard instructions edit --run",
  implement: "switchboard instructions edit --implement",
};

/** KindSection is the fields of one kind of instructions, global first, under its own heading. */
function KindSection({ kind, data }: { kind: InstructionKind; data: Instructions }) {
  const t = useMessages(messages);
  const k = t.kinds[kind];
  const items = data.items.filter((it) => (it.kind ?? "plan") === kind);
  return (
    <section data-kind={kind} aria-labelledby={`instructions-${kind}`} className="flex flex-col gap-sm">
      <div className="flex flex-col gap-xs">
        <h2 id={`instructions-${kind}`} className="text-body-md font-semibold">
          {k.heading}
        </h2>
        <p className="text-muted">{t.about(k.about)}</p>
        <p className="text-label-md text-muted">
          {t.host(
            <span className="font-mono text-mono-sm">{data.host}</span>,
            <code className="font-mono text-mono-sm">{commands[kind]}</code>,
          )}
        </p>
      </div>
      {items.map((it) => (
        <Field key={it.target} item={it} kind={kind} />
      ))}
    </section>
  );
}

/** The name of each language in itself, the same whichever language is shown, so a reader of either finds theirs. */
const languageNames: Record<Lang, string> = { ja: "日本語", en: "English" };

/** The choices of the language, in the order the display section lists them. */
const languageChoices: LangChoice[] = ["auto", "ja", "en"];

/**
 * DisplaySection is the part of the settings for everyone: the language of the inbox, as a radio group of Automatic
 * (the browser's language) and the two languages, each named in itself. A choice shows the inbox in it at once and is
 * kept in this browser only.
 */
function DisplaySection() {
  const t = useMessages(messages);
  const [choice, choose] = useLanguageChoice();
  return (
    <section aria-labelledby="display-heading" className="flex flex-col gap-sm">
      <h2 id="display-heading" className="text-body-md font-semibold">
        {t.display}
      </h2>
      <Card>
        <fieldset className="flex flex-col">
          <legend className="text-label-md font-medium">{t.language}</legend>
          {languageChoices.map((c) => (
            <label key={c} className="flex min-h-11 items-center gap-sm text-label-md">
              <input
                type="radio"
                name="language"
                value={c}
                checked={choice === c}
                onChange={() => choose(c)}
                aria-describedby={c === "auto" ? "language-auto-note" : undefined}
                className="size-5 shrink-0 accent-primary"
              />
              {c === "auto" ? <span>{t.auto}</span> : <span lang={c}>{languageNames[c]}</span>}
            </label>
          ))}
          <p id="language-auto-note" className="text-label-md text-muted">
            {t.autoNote}
          </p>
        </fieldset>
      </Card>
    </section>
  );
}

/**
 * SettingsPage, headed 設定 (Settings), shows everyone the display section first, then, to an admin or with the login
 * off, lists and adds the projects, and edits the additional instructions, global and per project: those the plan runs
 * get, those the runs that write files get, then those only the runs that implement an issue get.
 * Nothing is read until the login state is known. Someone logged in who is not an admin is told under the display
 * section that only an admin may change the projects and the instructions, and nothing is read (issue #175). A read
 * that works drops the error of the last one.
 */
export function SettingsPage() {
  const t = useMessages(messages);
  const { status, known } = useAuth();
  const denied = status.enabled && !status.admin;
  const [data, setData] = useState<Instructions | null>(null);
  const [error, setError] = useState<Failure | null>(null);
  usePageTitle(t.title);

  const load = () =>
    getInstructions().then(
      (d) => {
        setData(d);
        setError(null);
      },
      (err) => setError({ cause: err }),
    );
  useEffect(() => {
    if (known && !denied) load();
  }, [known, denied]);

  let rest: ReactNode;
  if (!known) rest = <Loading />;
  else if (denied) rest = <p className="text-muted">{t.adminOnly}</p>;
  else
    rest = (
      <>
        <ProjectsSection onAdded={load} />
        {error && <Alert>{failure(t, error.cause)}</Alert>}
        {!data && !error && <Loading />}
        {data && (
          <>
            <KindSection kind="plan" data={data} />
            <KindSection kind="run" data={data} />
            <KindSection kind="implement" data={data} />
          </>
        )}
      </>
    );
  return (
    <Shell>
      <h1 className="text-headline-md font-semibold">{t.title}</h1>
      <DisplaySection />
      {rest}
    </Shell>
  );
}
