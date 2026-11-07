import { useEffect, useState, type FormEvent } from "react";
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
import { instructionDraftOf, keepInstructionDraft } from "@/lib/instructionDrafts";
import { refText } from "@/lib/ref";
import { usePageTitle } from "@/lib/title";
import { toast } from "@/lib/toast";

function failure(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "too_large":
        return "32 KiBを超えているため、保存していません。";
      case "secret":
        return "秘密情報らしいものを含むため、保存していません。";
      case "unknown_project":
        return "設定に無いプロジェクトのため、保存していません。";
      case "forbidden":
        return "設定を変えられるのは管理者だけです。";
      case "unavailable":
        return "受信箱のサービス（switchboard inbox serve）に届きません。";
    }
  }
  return `失敗しました：${err instanceof Error ? err.message : String(err)}`;
}

/**
 * Field is one instruction file: its own text, its own save and its own refusal, so one refusal touches no other. A
 * save that worked is told in a toast that names the field.
 * While the text differs from the one last saved (at first, the one read), the field says it is not saved, and the
 * browser asks before the page unloads. The text not saved is kept in memory (lib/instructionDrafts.ts), so leaving
 * the settings and coming back finds it again.
 */
function Field({ item, kind }: { item: Instruction; kind: InstructionKind }) {
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
  const [error, setError] = useState<string | null>(null);
  const id = `instructions-${kind}-${item.target || "global"}`;
  const name = item.target ? refText(item.target) : "すべてのプロジェクト";

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await saveInstructions(item.target, kind, text);
      setSaved(text);
      toast.success(`${name}の${kinds[kind].noun}を保存しました`);
    } catch (err) {
      setError(failure(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section aria-labelledby={`${id}-label`}>
      <Card>
        <form onSubmit={submit} className="flex flex-col gap-sm">
          <label id={`${id}-label`} htmlFor={id} className={item.target ? undefined : "text-label-md font-medium"}>
            {item.target ? <IssueRef value={item.target} /> : name}
          </label>
          <Textarea
            id={id}
            value={text}
            onChange={(e) => setText(e.target.value)}
            disabled={saving}
          />
          {error && <Alert>{error}</Alert>}
          <div className="flex flex-wrap items-center justify-end gap-sm">
            {dirty && <span className="text-label-md text-muted">未保存の変更があります</span>}
            <Button type="submit" variant="secondary" disabled={saving}>
              {saving ? "保存しています" : "保存する"}
            </Button>
          </div>
        </form>
      </Card>
    </section>
  );
}

const kinds: Record<InstructionKind, { heading: string; noun: string; about: string; command: string }> = {
  plan: {
    heading: "計画の実行への追加の指示",
    noun: "計画の指示",
    about: "計画から始めたissueの、計画を立てる実行にだけ渡します。",
    command: "switchboard instructions edit",
  },
  run: {
    heading: "実装の実行への追加の指示",
    noun: "実装の指示",
    about: "ファイルを書く実行（実装、承認のあとの実装、CIやpushの失敗とbaseとの衝突を直す実行）に渡します。",
    command: "switchboard instructions edit --run",
  },
  implement: {
    heading: "実装の実行だけへの追加の指示",
    noun: "実装だけの指示",
    about:
      "直接の実装と承認のあとの実装にだけ、実装の実行への指示のあとに渡します。計画の実行と、直す実行には渡しません。スクリーンショットを撮る指示はここに書きます。",
    command: "switchboard instructions edit --implement",
  },
};

/** KindSection is the fields of one kind of instructions, global first, under its own heading. */
function KindSection({ kind, data }: { kind: InstructionKind; data: Instructions }) {
  const k = kinds[kind];
  const items = data.items.filter((it) => (it.kind ?? "plan") === kind);
  return (
    <section data-kind={kind} aria-labelledby={`instructions-${kind}`} className="flex flex-col gap-sm">
      <div className="flex flex-col gap-xs">
        <h2 id={`instructions-${kind}`} className="text-body-md font-semibold">
          {k.heading}
        </h2>
        <p className="text-muted">{k.about}すべてのプロジェクトの指示のあとに、そのプロジェクトの指示をつなげます。</p>
        <p className="text-label-md text-muted">
          端末 <span className="font-mono text-mono-sm">{data.host}</span>{" "}
          のファイルです。エージェントが別の端末で動くときは、その端末で{" "}
          <code className="font-mono text-mono-sm">{k.command}</code> を使ってください。
        </p>
      </div>
      {items.map((it) => (
        <Field key={it.target} item={it} kind={kind} />
      ))}
    </section>
  );
}

/**
 * SettingsPage, headed 設定, lists and adds the projects, and edits the additional instructions, global and per project: those the
 * plan runs get, those the runs that write files get, then those only the runs that implement an issue get.
 * Nothing is read until the login state is known. Someone logged in who is not an admin is told that only an admin
 * may change the settings, and nothing is read (issue #175). A read that works drops the error of the last one.
 */
export function SettingsPage() {
  const { status, known } = useAuth();
  const denied = status.enabled && !status.admin;
  const [data, setData] = useState<Instructions | null>(null);
  const [error, setError] = useState<string | null>(null);
  usePageTitle("設定");

  const load = () =>
    getInstructions().then(
      (d) => {
        setData(d);
        setError(null);
      },
      (err) => setError(failure(err)),
    );
  useEffect(() => {
    if (known && !denied) load();
  }, [known, denied]);

  if (!known) return <Shell><Loading /></Shell>;
  if (denied) {
    return (
      <Shell>
        <h1 className="text-headline-md font-semibold">設定</h1>
        <p className="text-muted">設定を変えられるのは管理者だけです。</p>
      </Shell>
    );
  }
  return (
    <Shell>
      <h1 className="text-headline-md font-semibold">設定</h1>
      <ProjectsSection onAdded={load} />
      {error && <Alert>{error}</Alert>}
      {!data && !error && <Loading />}
      {data && (
        <>
          <KindSection kind="plan" data={data} />
          <KindSection kind="run" data={data} />
          <KindSection kind="implement" data={data} />
        </>
      )}
    </Shell>
  );
}
