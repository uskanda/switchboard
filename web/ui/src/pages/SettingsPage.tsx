import { useEffect, useState, type FormEvent } from "react";
import { Shell } from "@/components/Shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, getInstructions, saveInstructions, type Instruction, type Instructions } from "@/lib/api";

function failure(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "too_large":
        return "32 KiBを超えているため、保存していません。";
      case "secret":
        return "秘密情報らしいものを含むため、保存していません。";
      case "unknown_project":
        return "設定に無いプロジェクトのため、保存していません。";
      case "unavailable":
        return "受信箱のサービス（switchboard inbox serve）に届きません。";
    }
  }
  return `失敗しました：${err instanceof Error ? err.message : String(err)}`;
}

/** Field is one instruction file: its own text, its own save and its own result, so one refusal touches no other. */
function Field({ item }: { item: Instruction }) {
  const [text, setText] = useState(item.content);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const id = `instructions-${item.target || "global"}`;
  const name = item.target || "すべてのプロジェクト";

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setSaved(false);
    setError(null);
    try {
      await saveInstructions(item.target, text);
      setSaved(true);
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
          <label id={`${id}-label`} htmlFor={id} className={item.target ? "font-mono text-mono-sm break-all" : "text-label-md font-medium"}>
            {name}
          </label>
          <Textarea
            id={id}
            value={text}
            onChange={(e) => {
              setText(e.target.value);
              setSaved(false);
            }}
            disabled={saving}
          />
          {error && <Alert>{error}</Alert>}
          <div className="flex items-center justify-end gap-md">
            {saved && (
              <p role="status" className="text-label-md text-muted">
                保存しました
              </p>
            )}
            <Button type="submit" variant="secondary" disabled={saving}>
              {saving ? "保存しています" : "保存する"}
            </Button>
          </div>
        </form>
      </Card>
    </section>
  );
}

/** SettingsPage edits the additional instructions that the plan runs get, global and per project. */
export function SettingsPage() {
  const [data, setData] = useState<Instructions | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getInstructions().then(setData, (err) => setError(failure(err)));
  }, []);

  return (
    <Shell>
      <div className="flex flex-col gap-xs">
        <h1 className="text-headline-md font-semibold">計画の実行への追加の指示</h1>
        <p className="text-muted">
          計画から始めたissueの、計画を立てる実行にだけ渡します。すべてのプロジェクトの指示のあとに、そのプロジェクトの指示をつなげます。
        </p>
        {data && (
          <p className="text-label-md text-muted">
            端末 <span className="font-mono text-mono-sm">{data.host}</span>{" "}
            のファイルです。エージェントが別の端末で動くときは、その端末で{" "}
            <code className="font-mono text-mono-sm">switchboard instructions edit</code> を使ってください。
          </p>
        )}
      </div>
      {error && <Alert>{error}</Alert>}
      {data && (
        <div className="flex flex-col gap-sm">
          {data.items.map((it) => (
            <Field key={it.target} item={it} />
          ))}
        </div>
      )}
    </Shell>
  );
}
