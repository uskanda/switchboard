import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { IssueRef } from "@/components/IssueRef";
import { Loading } from "@/components/Loading";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  ApiError,
  addProject,
  getProjects,
  planStartOf,
  type AddedProject,
  type PlanFirstMode,
  type Project,
  type Projects,
} from "@/lib/api";
import { defineMessages, errorText, useMessages } from "@/lib/i18n";
import { common } from "@/lib/i18n/common";
import { toast } from "@/lib/toast";

/** messages is the text of the projects of the settings: each project's state, the addition and its errors. */
const messages = defineMessages({
  ja: {
    heading: "プロジェクト",
    /** hostNote says whose projects these are; host is the inbox service's machine, in its own element. */
    hostNote: (host: ReactNode): ReactNode => <>端末 {host} のエージェントが扱うプロジェクトです。</>,
    /** How a project's ready label starts; "plan" is an older inbox service's plan_first, true. */
    start: {
      always: "必ず計画から始める",
      auto: "必要なら計画から始める",
      never: "すぐに実装する",
      plan: "計画から始める",
    },
    /** afterRef puts a project's start after its reference or its clone's place. */
    afterRef: (start: string) => `・${start}`,
    /** What each start the addition offers does. */
    modeNote: {
      auto: "要件が明確なら、質問と承認を省いて実装する",
      always: "承認を待つ計画を先に立てる",
      never: "計画を立てずに実装する",
    },
    aside: (note: string) => `（${note}）`,
    cloned: "クローン済み",
    notCloned:
      "クローンがありません。エージェントが10秒ほどで作ります。作られないときは、エージェントのトークンの対象にこのリポジトリが入っているかを確かめてください。",
    labelsUnread: (detail: string) => `状態ラベルを読めませんでした：${detail}`,
    labelsReady: "状態ラベルはそろっています",
    labelsMissing: (labels: string[]) => `足りない状態ラベル：${labels.join("、")}`,
    refLabel: "追加するプロジェクト",
    startLegend: "agent:ready で始めるとき",
    add: "追加する",
    adding: "追加しています",
    added: (ref: ReactNode): ReactNode => <>{ref} を追加しました</>,
    addedNote: "エージェントが10秒ほどでクローンし、状態ラベルを作ります。",
    webhook: "中継の端末で次を実行すると、webhook で早く着手します。登録するまでは、ポーリングで扱います：",
    /** howToEnable says how to turn the addition on; key is the setting's name, in its own element. */
    howToEnable: (key: ReactNode): ReactNode => (
      <>config.yaml に {key} （クローンを置く親のディレクトリ）を設定すると、ここからプロジェクトを追加できます。</>
    ),
    /** error names what went wrong by the inbox service's code; config_invalid adds the server's message. */
    error: {
      adding_disabled: "追加は無効です。config.yaml の inbox.project_root を設定してください。",
      invalid: "gh:owner/repo か gl:group/project の形で入力してください。",
      project_exists: "このプロジェクトは既に設定にあります。",
      forge_not_used:
        "この端末のプロジェクトが使っていないフォージです。config.yaml に手で足し、switchboard service install を実行し直してください。",
      repo_unreadable: "受信箱のトークンでこのリポジトリを読めません。名前と、トークンの権限を確かめてください。",
      path_taken: "別のプロジェクトが、同じ場所にクローンされています。",
      config_invalid: (detail: string) => `設定ファイルに誤りがあるため、追加していません：${detail}`,
      config_changed: "追加の途中で、設定ファイルが別に変更されました。もう一度追加してください。",
    },
    unreachable: common.ja.unreachable,
    failed: common.ja.failed,
  },
  en: {
    heading: "Projects",
    hostNote: (host) => <>The agent on host {host} works on these projects.</>,
    start: {
      always: "Always start with a plan",
      auto: "Start with a plan if needed",
      never: "Implement right away",
      plan: "Start with a plan",
    },
    afterRef: (start) => ` · ${start}`,
    modeNote: {
      auto: "skip questions and approval when the requirements are clear",
      always: "plan first and wait for approval",
      never: "skip the plan",
    },
    aside: (note) => ` (${note})`,
    cloned: "Cloned",
    notCloned:
      "Not cloned yet. The agent clones it in about 10 seconds. If it does not, check that the agent's token covers this repository.",
    labelsUnread: (detail) => `Could not read the state labels: ${detail}`,
    labelsReady: "All state labels exist",
    labelsMissing: (labels) => `Missing state labels: ${labels.join(", ")}`,
    refLabel: "Project to add",
    startLegend: "Starting with agent:ready",
    add: "Add",
    adding: "Adding",
    added: (ref) => <>Added {ref}</>,
    addedNote: "The agent clones it and creates the state labels in about 10 seconds.",
    webhook: "Run this on the relay's host to start sooner through a webhook. Until you do, the agent polls the project:",
    howToEnable: (key) => <>To add projects here, set {key} (the parent directory of the clones) in config.yaml.</>,
    error: {
      adding_disabled: "Adding is turned off. Set inbox.project_root in config.yaml.",
      invalid: "Enter gh:owner/repo or gl:group/project.",
      project_exists: "This project is already in the config.",
      forge_not_used:
        "No project on this host uses this forge. Add the project to config.yaml by hand, then run switchboard service install again.",
      repo_unreadable: "The inbox token cannot read this repository. Check the name and the token's permissions.",
      path_taken: "Another project is already cloned at the same path.",
      config_invalid: (detail) => `The project was not added because the config file has an error: ${detail}`,
      config_changed: "The config file changed while the project was being added. Add it again.",
    },
    unreachable: common.en.unreachable,
    failed: common.en.failed,
  },
});

type Text = (typeof messages)["ja"];

/** The starts the addition offers, the default first. */
const modes: PlanFirstMode[] = ["auto", "always", "never"];

const startOf = (t: Text, p: Project) => t.start[planStartOf(p) ?? "never"];

/** failure is what to say about an error of the inbox service, in the language shown. */
function failure(t: Text, err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "adding_disabled":
      case "invalid":
      case "project_exists":
      case "forge_not_used":
      case "repo_unreadable":
      case "path_taken":
      case "config_changed":
        return t.error[err.code];
      case "config_invalid":
        return t.error.config_invalid(err.message);
      case "unavailable":
        return t.unreachable;
    }
  }
  return t.failed(errorText(err));
}

/** Failure is an error kept until it is shown, so that it is said in the language shown at the time. */
type Failure = { cause: unknown };

/** ready is a project with no problem: cloned, with its state labels read and none missing. */
const ready = (p: Project) => p.cloned && !p.labels_error && p.missing_labels.length === 0;

/**
 * Row is one project. One with a problem is a card that says what it still needs before the agent runs it: a clone
 * and the state labels. One with no problem is a line of its reference and its start, with its clone's place in its
 * title.
 */
function Row({ project }: { project: Project }) {
  const t = useMessages(messages);
  if (ready(project)) {
    return (
      <li className="flex flex-wrap items-center gap-x-sm gap-y-xs px-lg" title={project.path}>
        <IssueRef value={project.ref} />
        <span className="text-label-md text-muted">{t.afterRef(startOf(t, project))}</span>
      </li>
    );
  }
  return (
    <li>
      <Card className="flex flex-col gap-xs">
        <p>
          <IssueRef value={project.ref} />
        </p>
        <p className="text-label-md text-muted">
          <span className="font-mono text-mono-sm break-all">{project.path}</span>
          {t.afterRef(startOf(t, project))}
        </p>
        <p className="text-label-md text-muted">
          {project.cloned ? t.cloned : t.notCloned}
        </p>
        <p className="text-label-md text-muted">
          {project.labels_error
            ? t.labelsUnread(project.labels_error)
            : project.missing_labels.length === 0
              ? t.labelsReady
              : t.labelsMissing(project.missing_labels)}
        </p>
      </Card>
    </li>
  );
}

/** AddForm asks for the reference and how the ready label starts only; the inbox service decides the rest. */
function AddForm({ onAdded }: { onAdded: (added: AddedProject) => void }) {
  const t = useMessages(messages);
  const [ref, setRef] = useState("");
  const [planFirst, setPlanFirst] = useState<PlanFirstMode>("auto");
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState<Failure | null>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setAdding(true);
    setError(null);
    try {
      onAdded(await addProject(ref.trim(), planFirst));
      setRef("");
    } catch (err) {
      setError({ cause: err });
    } finally {
      setAdding(false);
    }
  }

  return (
    <Card>
      <form onSubmit={submit} className="flex flex-col gap-sm">
        <label htmlFor="project-ref" className="text-label-md font-medium">
          {t.refLabel}
        </label>
        <Input
          id="project-ref"
          value={ref}
          onChange={(e) => setRef(e.target.value)}
          placeholder="gh:owner/repo"
          autoCapitalize="off"
          autoCorrect="off"
          spellCheck={false}
          className="font-mono"
          disabled={adding}
        />
        <fieldset className="flex flex-col" disabled={adding}>
          <legend className="text-label-md font-medium">{t.startLegend}</legend>
          {modes.map((mode) => (
            <label key={mode} className="flex min-h-11 items-center gap-sm text-label-md">
              <input
                type="radio"
                name="plan-first"
                value={mode}
                checked={planFirst === mode}
                onChange={() => setPlanFirst(mode)}
                className="size-5 shrink-0 accent-primary"
              />
              <span>
                {t.start[mode]}
                <span className="text-muted">{t.aside(t.modeNote[mode])}</span>
              </span>
            </label>
          ))}
        </fieldset>
        {error && <Alert>{failure(t, error.cause)}</Alert>}
        <div className="flex justify-end">
          <Button type="submit" variant="secondary" disabled={adding || ref.trim() === ""}>
            {adding ? t.adding : t.add}
          </Button>
        </div>
      </form>
    </Card>
  );
}

/**
 * ProjectsSection lists the projects of the inbox service's machine and, when enabled, adds one. An addition is told in
 * a toast; the webhook command, which is copied and run on the relay's machine, stays under the list.
 */
export function ProjectsSection({ onAdded }: { onAdded?: () => void }) {
  const t = useMessages(messages);
  const [data, setData] = useState<Projects | null>(null);
  const [error, setError] = useState<Failure | null>(null);
  const [webhook, setWebhook] = useState<string | null>(null);

  const load = () =>
    getProjects().then(
      (d) => {
        setData(d);
        setError(null);
      },
      (err) => setError({ cause: err }),
    );
  useEffect(() => {
    load();
  }, []);

  return (
    <section aria-labelledby="projects-heading" className="flex flex-col gap-sm">
      <div className="flex flex-col gap-xs">
        <h2 id="projects-heading" className="text-body-md font-semibold">
          {t.heading}
        </h2>
        {data && (
          <p className="text-label-md text-muted">
            {t.hostNote(<span className="font-mono text-mono-sm">{data.host}</span>)}
          </p>
        )}
      </div>
      {error && <Alert>{failure(t, error.cause)}</Alert>}
      {!data && !error && <Loading />}
      {data && (
        <ul className="flex flex-col gap-sm">
          {[...data.items.filter((p) => !ready(p)), ...data.items.filter(ready)].map((p) => (
            <Row key={p.ref} project={p} />
          ))}
        </ul>
      )}
      {webhook && (
        <p className="text-label-md text-muted">
          {t.webhook}
          <code className="block font-mono text-mono-sm break-all">{webhook}</code>
        </p>
      )}
      {data &&
        (data.enabled ? (
          <AddForm
            onAdded={(a: AddedProject) => {
              toast.success(t.added(<IssueRef value={a.project.ref} />), { description: t.addedNote });
              setWebhook(a.webhook ?? null);
              load();
              onAdded?.();
            }}
          />
        ) : (
          <p className="text-label-md text-muted">{t.howToEnable(<code className="font-mono text-mono-sm">inbox.project_root</code>)}</p>
        ))}
    </section>
  );
}
