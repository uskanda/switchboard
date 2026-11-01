import { useEffect, useState, type FormEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ApiError, addProject, getProjects, type AddedProject, type Project, type Projects } from "@/lib/api";

function failure(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "adding_disabled":
        return "追加は無効です。config.yaml の inbox.project_root を設定してください。";
      case "invalid":
        return "gh:owner/repo か gl:group/project の形で入力してください。";
      case "project_exists":
        return "このプロジェクトは既に設定にあります。";
      case "forge_not_used":
        return "この端末のプロジェクトが使っていないフォージです。config.yaml に手で足し、switchboard service install を実行し直してください。";
      case "repo_unreadable":
        return "受信箱のトークンでこのリポジトリを読めません。名前と、トークンの権限を確かめてください。";
      case "path_taken":
        return "別のプロジェクトが、同じ場所にクローンされています。";
      case "config_invalid":
        return `設定ファイルに誤りがあるため、追加していません：${err.message}`;
      case "config_changed":
        return "追加の途中で、設定ファイルが別に変更されました。もう一度追加してください。";
      case "unavailable":
        return "受信箱のサービス（switchboard inbox serve）に届きません。";
    }
  }
  return `失敗しました：${err instanceof Error ? err.message : String(err)}`;
}

/** Row is one project and what it still needs before the agent runs it: a clone and the state labels. */
function Row({ project }: { project: Project }) {
  return (
    <li>
      <Card className="flex flex-col gap-xs">
        <p className="font-mono text-mono-sm break-all">{project.ref}</p>
        <p className="text-label-md text-muted">
          <span className="font-mono text-mono-sm break-all">{project.path}</span>・
          {project.plan_first ? "計画から始める" : "すぐに実装する"}
        </p>
        <p className="text-label-md text-muted">
          {project.cloned
            ? "クローン済み"
            : "クローンがありません。エージェントが10秒ほどで作ります。作られないときは、エージェントのトークンの対象にこのリポジトリが入っているかを確かめてください。"}
        </p>
        <p className="text-label-md text-muted">
          {project.labels_error
            ? `状態ラベルを読めませんでした：${project.labels_error}`
            : project.missing_labels.length === 0
              ? "状態ラベルはそろっています"
              : `足りない状態ラベル：${project.missing_labels.join("、")}`}
        </p>
      </Card>
    </li>
  );
}

/** AddForm asks for the reference and plan first only; the inbox service decides the rest. */
function AddForm({ onAdded }: { onAdded: (added: AddedProject) => void }) {
  const [ref, setRef] = useState("");
  const [planFirst, setPlanFirst] = useState(true);
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setAdding(true);
    setError(null);
    try {
      onAdded(await addProject(ref.trim(), planFirst));
      setRef("");
    } catch (err) {
      setError(failure(err));
    } finally {
      setAdding(false);
    }
  }

  return (
    <Card>
      <form onSubmit={submit} className="flex flex-col gap-sm">
        <label htmlFor="project-ref" className="text-label-md font-medium">
          追加するプロジェクト
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
        <label className="flex min-h-11 items-center gap-sm text-label-md">
          <input
            type="checkbox"
            checked={planFirst}
            onChange={(e) => setPlanFirst(e.target.checked)}
            disabled={adding}
            className="size-5 accent-primary"
          />
          計画から始める（agent:ready で、承認を待つ計画を先に立てる）
        </label>
        {error && <Alert>{error}</Alert>}
        <div className="flex justify-end">
          <Button type="submit" variant="secondary" disabled={adding || ref.trim() === ""}>
            {adding ? "追加しています" : "追加する"}
          </Button>
        </div>
      </form>
    </Card>
  );
}

/** ProjectsSection lists the projects of the inbox service's machine and, when enabled, adds one. */
export function ProjectsSection({ onAdded }: { onAdded?: () => void }) {
  const [data, setData] = useState<Projects | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [added, setAdded] = useState<AddedProject | null>(null);

  const load = () => getProjects().then(setData, (err) => setError(failure(err)));
  useEffect(() => {
    load();
  }, []);

  return (
    <section aria-labelledby="projects-heading" className="flex flex-col gap-sm">
      <div className="flex flex-col gap-xs">
        <h2 id="projects-heading" className="text-headline-md font-semibold">
          プロジェクト
        </h2>
        {data && (
          <p className="text-label-md text-muted">
            端末 <span className="font-mono text-mono-sm">{data.host}</span> のエージェントが扱うプロジェクトです。
          </p>
        )}
      </div>
      {error && <Alert>{error}</Alert>}
      {data && (
        <ul className="flex flex-col gap-sm">
          {data.items.map((p) => (
            <Row key={p.ref} project={p} />
          ))}
        </ul>
      )}
      {added && (
        <div role="status" className="flex flex-col gap-xs text-label-md text-muted">
          <p>
            <span className="font-mono text-mono-sm">{added.project.ref}</span> を追加しました。エージェントが10秒ほどでクローンし、状態ラベルを作ります。
          </p>
          {added.webhook && (
            <p>
              中継を使っているので、中継の端末で webhook を登録してください：
              <code className="block font-mono text-mono-sm break-all">{added.webhook}</code>
            </p>
          )}
        </div>
      )}
      {data &&
        (data.enabled ? (
          <AddForm
            onAdded={(a) => {
              setAdded(a);
              load();
              onAdded?.();
            }}
          />
        ) : (
          <p className="text-label-md text-muted">
            config.yaml に <code className="font-mono text-mono-sm">inbox.project_root</code>{" "}
            （クローンを置く親のディレクトリ）を設定すると、ここからプロジェクトを追加できます。
          </p>
        ))}
    </section>
  );
}
