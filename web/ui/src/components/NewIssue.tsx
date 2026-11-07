import { useCallback, useId, useRef, useState, type FormEvent } from "react";
import { IssueRef } from "@/components/IssueRef";
import { Loading } from "@/components/Loading";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, createIssue, getIssueTargets, planStartOf, type IssueTargets, type PlanStart, type StartKind } from "@/lib/api";
import { useInbox } from "@/lib/inbox";
import { refText, splitRef } from "@/lib/ref";
import { toast } from "@/lib/toast";

/** What each kind of start does, before the label it stands for. */
const startText: Record<StartKind, string> = {
  ready: "プロジェクトの設定どおりに始める",
  plan: "計画から始める",
  go: "計画なしで実装する",
  none: "ラベルなし（実装開始しない）",
};

/** What the ready label does in the chosen project; "plan" is an older inbox service's plan_first, true. */
const readyNotes: Record<PlanStart, string> = {
  always: "（このプロジェクトは必ず計画から始めます）",
  auto: "（このプロジェクトは必要なら計画から始めます）",
  never: "（このプロジェクトはすぐに実装します）",
  plan: "（このプロジェクトは計画から始めます）",
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
 * label or none; the list puts it in its top row. The ready start says whether the chosen project starts with a plan. The issue is opened with the inbox
 * token, so the agent takes it up when it has a start label (ADR-040); one without waits until the person puts one on
 * it (ADR-049). Once it is opened the dialog closes and a success toast links to it on the forge. Until the agent
 * takes up an issue with a start label, the list shows a running row of it, marked as being processed
 * (lib/pending.ts); one without gets no such row, since the agent does not take it up.
 */
export function NewIssue() {
  const { markPending } = useInbox();
  const dialog = useRef<HTMLDialogElement | null>(null);
  // The list of projects is put into the dialog, which a modal dialog needs (components/ui/select.tsx).
  const [dialogEl, setDialogEl] = useState<HTMLDialogElement | null>(null);
  const dialogRef = useCallback((el: HTMLDialogElement | null) => {
    dialog.current = el;
    setDialogEl(el);
  }, []);
  const id = useId();
  const [targets, setTargets] = useState<IssueTargets | null>(null);
  const [project, setProject] = useState("");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [start, setStart] = useState<StartKind>("ready");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
      const hash = out.ref.lastIndexOf("#");
      if (start !== "none") {
        markPending(out.ref, "create", {
          title: title.trim(),
          url: out.url,
          project: splitRef(hash < 0 ? out.ref : out.ref.slice(0, hash)).path,
          number: hash < 0 ? 0 : Number(out.ref.slice(hash + 1)) || 0,
        });
      }
      toast.success(
        <>
          <IssueRef value={out.ref} /> を作りました
        </>,
        {
          action: (
            <a href={out.url} target="_blank" rel="noreferrer" className="self-start underline focus-visible:outline-2 focus-visible:outline-primary">
              フォージで開く
            </a>
          ),
        },
      );
      setTitle("");
      setBody("");
      dialog.current?.close();
    } catch (err) {
      setError(failure(err));
    } finally {
      setSending(false);
    }
  }

  // What the ready label does in the chosen project; an older inbox service does not say, and nothing is added.
  const target = targets?.items.find((it) => it.ref === project);
  const planStart = target && planStartOf(target);
  const readyNote = planStart ? readyNotes[planStart] : "";
  const ready = targets !== null && project !== "" && targets.starts.some((st) => st.kind === start);
  const canSend = ready && title.trim() !== "" && !sending;

  return (
    <>
      <Button type="button" variant="secondary" onClick={open}>
        新規issue
      </Button>
      {/* While the issue is being created, Escape leaves the dialog open: a failure is told in it (issue #173). */}
      <Dialog ref={dialogRef} aria-labelledby={`${id}-heading`} onCancel={(e) => sending && e.preventDefault()}>
        <form onSubmit={submit} className="flex flex-col gap-md">
          <h2 id={`${id}-heading`} className="text-headline-md font-semibold">
            新規issue
          </h2>
          {!targets && !error && <Loading />}
          <div className="flex flex-col gap-xs">
            <label id={`${id}-project-label`} htmlFor={`${id}-project`} className="text-label-md font-medium">
              プロジェクト
            </label>
            <Select value={project} onValueChange={setProject} disabled={sending || !targets}>
              <SelectTrigger id={`${id}-project`} aria-labelledby={`${id}-project-label`}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent container={dialogEl}>
                {targets?.items.map((it) => (
                  <SelectItem key={it.ref} value={it.ref} aria-label={`${refText(it.ref)}（担当：${it.host}）`}>
                    <IssueRef value={it.ref} />（担当：{it.host}）
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
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
                  {startText[st.kind]}
                  {st.label !== "" && (
                    <>
                      （<span className="font-mono text-mono-sm">{st.label}</span>）
                    </>
                  )}
                  {st.kind === "ready" && readyNote && <span className="text-muted">{readyNote}</span>}
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
      </Dialog>
    </>
  );
}
