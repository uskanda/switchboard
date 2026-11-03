import { useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { ApiError, type CheckStatus, merge, type MergeStatus, mergeStatus } from "@/lib/api";
import { useInbox } from "@/lib/inbox";
import { toast } from "@/lib/toast";

/** changeName is how the forge names the change request: "PR #9" on GitHub, "MR !9" on GitLab. */
export function changeName(ref: string, number: number) {
  return ref.startsWith("gl:") ? `MR !${number}` : `PR #${number}`;
}

function failure(name: string, err: unknown): { message: string; description?: string } {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "stale":
        return { message: `${name}かissueが、画面を読んだ後に変わっています。開き直してください。`, description: err.message };
      case "not_mergeable":
        return { message: `フォージが${name}のマージを断りました。`, description: err.message };
      case "forbidden":
        return {
          message: "受信箱のトークンに、マージする権限がありません。",
          description: "GitHubでは Pull requests と Contents の書き込みの権限が要ります。GitLabでは api のスコープが要ります。",
        };
      case "unavailable":
        return { message: "受信箱のサービス（switchboard inbox serve）に届きません。" };
    }
  }
  return { message: `${name}をマージできませんでした。`, description: err instanceof Error ? err.message : String(err) };
}

/** Read is the dialog's reading of the change's status: under way, done, or failed with what to say. */
type Read = { kind: "reading" } | { kind: "read"; status: MergeStatus } | { kind: "failed"; message: string; description?: string };

function readFailure(err: unknown): { message: string; description?: string } {
  const message = "マージの前の状態を読めませんでした。マージはこのまま押せます。";
  if (err instanceof ApiError) {
    switch (err.code) {
      case "stale":
        return { message: "issueかPR/MRが、画面を読んだ後に変わっています。開き直してください。", description: err.message };
      case "unavailable":
        return { message, description: "受信箱のサービス（switchboard inbox serve）に届きません。" };
    }
    return { message, description: err.message };
  }
  return { message, description: err instanceof Error ? err.message : String(err) };
}

/** webPage is url when it is an http(s) address, which a check's link may open; the forge's data is not trusted. */
function webPage(url: string): string | undefined {
  return /^https?:\/\//i.test(url) ? url : undefined;
}

const checkWord: Record<CheckStatus["status"], string> = { success: "成功", failure: "失敗", running: "実行中", other: "その他" };

/**
 * ChangeStatus shows where the change stands before the merge: the conflict, the commits added after the page was
 * read, and the CI of the commit. It only informs; the forge still decides whether the merge goes through (ADR-047).
 * The checks' names are the forge's data and are shown as text.
 */
function ChangeStatus({ name, read }: { name: string; read: Read }) {
  if (read.kind === "reading") {
    return (
      <p className="flex items-center gap-xs text-muted">
        <Spinner />
        確かめています
      </p>
    );
  }
  if (read.kind === "failed") {
    return (
      <div className="text-muted">
        <p>{read.message}</p>
        {read.description && <p className="break-words font-mono text-mono-sm">{read.description}</p>}
      </div>
    );
  }
  const st = read.status;
  const count = (s: CheckStatus["status"]) => st.checks.filter((c) => c.status === s).length;
  const listed = st.checks.filter((c) => c.status === "failure" || c.status === "running");
  return (
    <ul className="flex flex-col gap-xs">
      {st.state === "merged" && <li className="font-medium text-attention">{name}は既にマージされています。</li>}
      {st.state === "closed" && <li className="font-medium text-attention">{name}は閉じられています。</li>}
      {st.head_moved && <li className="font-medium text-attention">画面を読んだ後にコミットが増えています。</li>}
      <li>
        {st.mergeable === "clean" && <span className="rounded-sm bg-success-container px-xs text-on-success-container">衝突なし</span>}
        {st.mergeable === "conflict" && <span className="rounded-sm bg-error-container px-xs font-medium text-on-error-container">衝突あり</span>}
        {st.mergeable === "computing" && <span className="text-muted">衝突の有無は、フォージが計算中です。</span>}
      </li>
      {st.ci === "forbidden" && (
        <li className="text-muted">受信箱のトークンにCIを読む権限がありません（GitHubでは Actions と Commit statuses の読み取り）。</li>
      )}
      {st.ci === "none" && <li className="text-muted">CIはありません</li>}
      {st.ci === "ok" && (
        <li>
          <p className={count("failure") > 0 ? "font-medium text-error" : undefined}>
            {`CI：成功 ${count("success")}／失敗 ${count("failure")}／実行中 ${count("running")}`}
          </p>
          {listed.length > 0 && (
            <ul className="mt-xs flex flex-col">
              {listed.map((c, i) => {
                const href = webPage(c.url);
                return (
                  <li key={`${i}-${c.name}`} className="flex min-h-11 items-center gap-xs break-all">
                    <span className={c.status === "failure" ? "shrink-0 text-error" : "shrink-0 text-muted"}>{checkWord[c.status]}</span>
                    {href ? (
                      <a href={href} target="_blank" rel="noreferrer" className="underline">
                        {c.name}
                      </a>
                    ) : (
                      <span>{c.name}</span>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </li>
      )}
    </ul>
  );
}

/**
 * MergeButton merges the pull or merge request of a done issue: the one its latest done post names, at the commit
 * the page showed. It asks first in a dialog, since a merge cannot be undone; the outcome is told in a toast. The
 * issue itself is closed by the change request's "Closes #N", and the list drops it on its next read (ADR-043).
 * Until then the issue is marked as being merged (lib/pending.ts), and the button cannot be pressed.
 * Each time the dialog opens, it reads the change's conflict and CI anew; what it reads never holds the merge back.
 */
export function MergeButton({ issueRef, number, sha, url }: { issueRef: string; number: number; sha: string; url?: string }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const backOut = useRef<HTMLButtonElement>(null);
  const id = useId();
  const [sending, setSending] = useState(false);
  const [read, setRead] = useState<Read | null>(null);
  // reads counts the openings: a reading that comes back after the dialog closed, or opened again, is dropped.
  const reads = useRef(0);
  const { view, markPending } = useInbox();
  const merging = view?.some((it) => it.ref === issueRef && it.pending === "merge") ?? false;
  const name = changeName(issueRef, number);

  // The dialog would put the focus on its first link, the change request's; a merge cannot be undone, so the focus
  // starts on backing out instead. React's autoFocus focuses on mount, while the dialog is closed, so it is moved here.
  function open() {
    const n = ++reads.current;
    setRead({ kind: "reading" });
    dialog.current?.showModal();
    backOut.current?.focus();
    mergeStatus(issueRef, sha).then(
      (status) => reads.current === n && setRead({ kind: "read", status }),
      (err: unknown) => reads.current === n && setRead({ kind: "failed", ...readFailure(err) }),
    );
  }

  function closed() {
    reads.current++;
    setRead(null);
  }

  async function confirm() {
    setSending(true);
    try {
      await merge(issueRef, sha);
      markPending(issueRef, "merge");
      toast.success(`${name}をマージしました`, { description: "issueは、PR/MRの Closes でcloseされると一覧から消えます。" });
    } catch (err) {
      const f = failure(name, err);
      toast.error(f.message, { description: f.description });
    } finally {
      setSending(false);
      dialog.current?.close();
    }
  }

  return (
    <>
      <Button type="button" variant="secondary" onClick={open} disabled={sending || merging} aria-label={`${name}をマージ`}>
        {sending && <Spinner />}
        マージ
      </Button>
      <Dialog ref={dialog} aria-labelledby={`${id}-heading`} onCancel={(e) => sending && e.preventDefault()} onClose={closed}>
        <div className="flex flex-col gap-md">
          <h2 id={`${id}-heading`} className="text-headline-md font-semibold">
            {name}をマージしますか
          </h2>
          <p className="break-words">
            {name}のdraftを解除してマージします。取り消せません。
          </p>
          {url && (
            <a href={url} target="_blank" rel="noreferrer" className="inline-flex min-h-11 items-center self-start break-all font-mono text-mono-sm underline">
              {url}
            </a>
          )}
          {read && (
            <section aria-label={`${name}の状態`} aria-live="polite" className="border-t border-outline pt-md">
              <ChangeStatus name={name} read={read} />
            </section>
          )}
          <div className="flex flex-wrap justify-end gap-sm">
            <Button ref={backOut} type="button" variant="secondary" onClick={() => dialog.current?.close()} disabled={sending}>
              やめる
            </Button>
            <Button type="button" onClick={() => void confirm()} disabled={sending}>
              {sending && <Spinner />}
              {sending ? "マージしています" : "マージする"}
            </Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
