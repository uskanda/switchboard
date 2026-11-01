import { useId, useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, createIssue, getIssueTargets, type CreatedIssue, type IssueTargets, type StartKind } from "@/lib/api";

/** What each kind of start does, before the label it stands for. */
const startText: Record<StartKind, string> = {
  ready: "プロジェクトの設定どおりに始める",
  plan: "計画から始める",
  go: "計画なしで実装する",
};

/** The most characters a title may have; the inbox service refuses a longer one. */
const maxTitle = 256;

function failure(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "unknown_project":
        return "このプロジェクトは設定にありません。設定が変わったかもしれません。開き直してください。";
      case "invalid":
        return `入力を確かめてください：${err.message}`;
      case "too_large":
        return "本文が大きすぎます。1MiBまでにしてください。";
      case "forge":
        return `フォージでissueを作れませんでした：${err.message}`;
      case "unavailable":
        return "受信箱のサービス（switchboard inbox serve）に届きません。";
    }
  }
  return `失敗しました：${err instanceof Error ? err.message : String(err)}`;
}

/**
 * NewIssue is the button above the list that opens a dialog for a new issue in any configured project, with a start
 * label. The issue is opened with the inbox token, so the agent takes it up (ADR-040). Once it is opened the dialog
 * closes and a notice above the list links to it on the forge.
 */
export function NewIssue() {
  const dialog = useRef<HTMLDialogElement>(null);
  const id = useId();
  const [targets, setTargets] = useState<IssueTargets | null>(null);
  const [project, setProject] = useState("");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [start, setStart] = useState<StartKind>("ready");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [created, setCreated] = useState<CreatedIssue | null>(null);

  function open() {
    setError(null);
    dialog.current?.showModal();
    // The projects are read on each opening, so a project added meanwhile can be chosen.
    getIssueTargets().then(
      (t) => {
        setTargets(t);
        setProject((p) => (t.items.some((it) => it.ref === p) ? p : (t.items[0]?.ref ?? "")));
        setStart((s) => (t.starts.some((st) => st.kind === s) ? s : (t.starts.find((st) => st.kind === "ready") ?? t.starts[0])?.kind ?? "ready"));
      },
      (err) => setError(failure(err)),
    );
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSending(true);
    setError(null);
    try {
      const out = await createIssue({ ref: project, title: title.trim(), body, start });
      setCreated(out);
      setTitle("");
      setBody("");
      dialog.current?.close();
    } catch (err) {
      setError(failure(err));
    } finally {
      setSending(false);
    }
  }

  const ready = targets !== null && project !== "" && targets.starts.some((st) => st.kind === start);
  const canSend = ready && title.trim() !== "" && !sending;

  return (
    <div className="flex flex-col gap-sm">
      <div className="flex justify-end">
        <Button type="button" variant="secondary" onClick={open}>
          新規issue
        </Button>
      </div>
      <div aria-live="polite">
        {created && (
          <div role="status" className="flex items-start gap-sm rounded-md border border-outline bg-surface-raised p-sm text-label-md">
            <p className="flex-1 break-words">
              <span className="font-mono text-mono-sm break-all">{created.ref}</span> を作りました。
              <a href={created.url} target="_blank" rel="noreferrer" className="underline focus-visible:outline-2 focus-visible:outline-primary">
                フォージで開く
              </a>
            </p>
            <button
              type="button"
              aria-label="知らせを閉じる"
              onClick={() => setCreated(null)}
              className="-m-xs inline-flex size-11 shrink-0 items-center justify-center rounded-md text-muted focus-visible:outline-2 focus-visible:outline-primary"
            >
              ×
            </button>
          </div>
        )}
      </div>
      <dialog
        ref={dialog}
        aria-labelledby={`${id}-heading`}
        className="m-auto max-h-[calc(100dvh-2*var(--spacing-lg))] w-[calc(100%-2*var(--spacing-lg))] max-w-[720px] overflow-y-auto rounded-md border border-outline bg-surface p-lg text-on-surface backdrop:bg-black/40"
      >
        <form onSubmit={submit} className="flex flex-col gap-md">
          <h2 id={`${id}-heading`} className="text-headline-md font-semibold">
            新規issue
          </h2>
          <div className="flex flex-col gap-xs">
            <label htmlFor={`${id}-project`} className="text-label-md font-medium">
              プロジェクト
            </label>
            <select
              id={`${id}-project`}
              value={project}
              onChange={(e) => setProject(e.target.value)}
              disabled={sending || !targets}
              className="h-11 w-full min-w-0 rounded-md border border-outline bg-surface-raised px-md font-mono text-mono-sm focus-visible:outline-2 focus-visible:outline-primary"
            >
              {targets?.items.map((it) => (
                <option key={it.ref} value={it.ref}>
                  {it.ref}（担当：{it.host}）
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-xs">
            <label htmlFor={`${id}-title`} className="text-label-md font-medium">
              タイトル
            </label>
            <Input id={`${id}-title`} value={title} onChange={(e) => setTitle(e.target.value)} maxLength={maxTitle} required disabled={sending} />
          </div>
          <div className="flex flex-col gap-xs">
            <label htmlFor={`${id}-body`} className="text-label-md font-medium">
              本文
            </label>
            <Textarea id={`${id}-body`} value={body} onChange={(e) => setBody(e.target.value)} placeholder="Markdownで書けます（任意）" disabled={sending} />
          </div>
          <fieldset className="flex flex-col">
            <legend className="text-label-md font-medium">開始ラベル</legend>
            {targets?.starts.map((st) => (
              <label key={st.kind} className="flex min-h-11 items-center gap-sm text-label-md">
                <input
                  type="radio"
                  name={`${id}-start`}
                  value={st.kind}
                  checked={start === st.kind}
                  onChange={() => setStart(st.kind)}
                  disabled={sending}
                  className="size-5 accent-primary"
                />
                <span>
                  {startText[st.kind]}（<span className="font-mono text-mono-sm">{st.label}</span>）
                </span>
              </label>
            ))}
          </fieldset>
          {error && <Alert>{error}</Alert>}
          <div className="flex flex-wrap justify-end gap-sm">
            <Button type="button" variant="secondary" onClick={() => dialog.current?.close()} disabled={sending}>
              やめる
            </Button>
            <Button type="submit" disabled={!canSend}>
              {sending ? "作成しています" : "作成する"}
            </Button>
          </div>
        </form>
      </dialog>
    </div>
  );
}
