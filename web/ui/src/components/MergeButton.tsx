import { useEffect, useId, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { ApiError, cancelAutoMerge, changeName, type CheckStatus, merge, type MergeStatus, mergeStatus } from "@/lib/api";
import { defineMessages, errorText, useMessages } from "@/lib/i18n";
import { common } from "@/lib/i18n/common";
import { useInbox } from "@/lib/inbox";
import { toast } from "@/lib/toast";

export { changeName };

/** messages is the text of the merge, its dialog and status, and the automatic merge's badge and cancel. */
const messages = defineMessages({
  ja: {
    merge: "マージ",
    mergeLabel: (name: string) => `${name}をマージ`,
    autoMergePending: "自動マージ待ち",
    autoMergePendingLabel: (name: string) => `${name}は自動マージ待ち`,
    askMerge: (name: string) => `${name}をマージしますか`,
    mergeExplained: (name: string) => `${name}のdraftを解除してマージします。取り消せません。`,
    statusLabel: (name: string) => `${name}の状態`,
    cancel: common.ja.cancel,
    confirmMerge: "マージする",
    merging: "マージしています",
    mergeLater: (name: string) => `${name}は衝突しているため、エージェントが直したあと自動でマージします`,
    mergeLaterHint: "やめるときは、issueを開いて「自動マージを取り消す」を押してください。",
    merged: (name: string) => `${name}をマージしました`,
    closesHint: "issueは、PR/MRの Closes でcloseされると一覧から消えます。",
    stale: (name: string) => `${name}かissueが、画面を読んだ後に変わっています。開き直してください。`,
    refused: (name: string) => `フォージが${name}のマージを断りました。`,
    forbidden: "受信箱のトークンに、マージする権限がありません。",
    forbiddenHint: "GitHubでは Pull requests と Contents の書き込みの権限が要ります。GitLabでは api のスコープが要ります。",
    unreachable: common.ja.unreachable,
    mergeFailed: (name: string) => `${name}をマージできませんでした。`,
    readFailed: "マージの前の状態を読めませんでした。マージはこのまま押せます。",
    readStale: "issueかPR/MRが、画面を読んだ後に変わっています。開き直してください。",
    checking: "確かめています",
    alreadyMerged: (name: string) => `${name}は既にマージされています。`,
    closed: (name: string) => `${name}は閉じられています。`,
    headMoved: "画面を読んだ後にコミットが増えています。",
    clean: "衝突なし",
    conflict: "衝突あり",
    computing: "衝突の有無は、フォージが計算中です。",
    ciForbidden: "受信箱のトークンにCIを読む権限がありません（GitHubでは Actions と Commit statuses の読み取り）。",
    ciNone: "CIはありません",
    ciSummary: (success: number, failure: number, running: number) => `CI：成功 ${success}／失敗 ${failure}／実行中 ${running}`,
    /** check names the state of one check of the CI. */
    check: { success: "成功", failure: "失敗", running: "実行中", other: "その他" },
    cancelAutoMerge: "自動マージを取り消す",
    askCancel: (name: string) => `${name}の自動マージを取り消しますか`,
    cancelExplained: (name: string) => `エージェントが衝突を直しても、${name}はマージされません。マージするときは、もう一度マージを押してください。`,
    /** keep backs out of cancelling; English names it apart from the cancel it backs out of. */
    keep: common.ja.cancel,
    confirmCancel: "取り消す",
    cancelling: "取り消しています",
    cancelled: (name: string) => `${name}の自動マージを取り消しました`,
    notPending: (name: string) => `${name}の自動マージは、もう待っていません。`,
    notPendingHint: "マージされたか、やめた後です。開き直してください。",
    cancelFailed: (name: string) => `${name}の自動マージを取り消せませんでした。`,
  },
  en: {
    merge: "Merge",
    mergeLabel: (name) => `Merge ${name}`,
    autoMergePending: "Auto-merge pending",
    autoMergePendingLabel: (name) => `Auto-merge pending for ${name}`,
    askMerge: (name) => `Merge ${name}?`,
    mergeExplained: (name) => `This takes ${name} out of draft and merges it. You cannot undo it.`,
    statusLabel: (name) => `Status of ${name}`,
    cancel: common.en.cancel,
    confirmMerge: "Merge",
    merging: "Merging",
    mergeLater: (name) => `${name} has conflicts, so it will be merged automatically once the agent fixes them`,
    mergeLaterHint: 'To stop it, open the issue and press "Cancel auto-merge".',
    merged: (name) => `${name} was merged`,
    closesHint: 'The issue leaves the list once "Closes" in the pull or merge request closes it.',
    stale: (name) => `${name} or the issue was updated after the page loaded. Reload the page.`,
    refused: (name) => `The forge refused to merge ${name}.`,
    forbidden: "The inbox token has no permission to merge.",
    forbiddenHint: "On GitHub it needs write access to Pull requests and Contents. On GitLab it needs the api scope.",
    unreachable: common.en.unreachable,
    mergeFailed: (name) => `Could not merge ${name}.`,
    readFailed: "Could not read the status before the merge. You can still merge.",
    readStale: "The issue or its pull or merge request was updated after the page loaded. Reload the page.",
    checking: "Checking",
    alreadyMerged: (name) => `${name} is already merged.`,
    closed: (name) => `${name} is closed.`,
    headMoved: "Commits were added after the page loaded.",
    clean: "No conflicts",
    conflict: "Has conflicts",
    computing: "The forge is still checking for conflicts.",
    ciForbidden: "The inbox token has no permission to read CI (on GitHub: read access to Actions and Commit statuses).",
    ciNone: "No CI",
    ciSummary: (success, failure, running) => `CI: ${success} passed, ${failure} failed, ${running} running`,
    check: { success: "Passed", failure: "Failed", running: "Running", other: "Other" },
    cancelAutoMerge: "Cancel auto-merge",
    askCancel: (name) => `Cancel the auto-merge of ${name}?`,
    cancelExplained: (name) => `${name} will not be merged, even once the agent fixes the conflict. To merge it, press "Merge" again.`,
    keep: "Keep auto-merge",
    confirmCancel: "Cancel auto-merge",
    cancelling: "Cancelling",
    cancelled: (name) => `Auto-merge of ${name} was cancelled`,
    notPending: (name) => `Auto-merge of ${name} is no longer pending.`,
    notPendingHint: "It was merged or cancelled. Reload the page.",
    cancelFailed: (name) => `Could not cancel the auto-merge of ${name}.`,
  },
});

type Text = (typeof messages)["ja"];

function failure(t: Text, name: string, err: unknown): { message: string; description?: string } {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "stale":
        return { message: t.stale(name), description: err.message };
      case "not_mergeable":
        return { message: t.refused(name), description: err.message };
      case "forbidden":
        return { message: t.forbidden, description: t.forbiddenHint };
      case "unavailable":
        return { message: t.unreachable };
    }
  }
  return { message: t.mergeFailed(name), description: errorText(err) };
}

/**
 * Read is the dialog's reading of the change's status: under way, done, or failed. A failure keeps its error, so its
 * text follows the language shown.
 */
type Read = { kind: "reading" } | { kind: "read"; status: MergeStatus } | { kind: "failed"; err: unknown };

function readFailure(t: Text, err: unknown): { message: string; description?: string } {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "stale":
        return { message: t.readStale, description: err.message };
      case "unavailable":
        return { message: t.readFailed, description: t.unreachable };
    }
    return { message: t.readFailed, description: err.message };
  }
  return { message: t.readFailed, description: errorText(err) };
}

/** webPage is url when it is an http(s) address, which a check's link may open; the forge's data is not trusted. */
function webPage(url: string): string | undefined {
  return /^https?:\/\//i.test(url) ? url : undefined;
}

/**
 * ChangeStatus shows where the change stands before the merge: the conflict, the commits added after the page was
 * read, and the CI of the commit. It only informs; the forge still decides whether the merge goes through (ADR-050).
 * The checks' names are the forge's data and are shown as text.
 */
function ChangeStatus({ name, read }: { name: string; read: Read }) {
  const t = useMessages(messages);
  if (read.kind === "reading") {
    return (
      <p className="flex items-center gap-xs text-muted">
        <Spinner />
        {t.checking}
      </p>
    );
  }
  if (read.kind === "failed") {
    const f = readFailure(t, read.err);
    return (
      <div className="text-muted">
        <p>{f.message}</p>
        {f.description && <p className="break-words font-mono text-mono-sm">{f.description}</p>}
      </div>
    );
  }
  const st = read.status;
  const count = (s: CheckStatus["status"]) => st.checks.filter((c) => c.status === s).length;
  const listed = st.checks.filter((c) => c.status === "failure" || c.status === "running");
  return (
    <ul className="flex flex-col gap-xs">
      {st.state === "merged" && <li className="font-medium text-attention">{t.alreadyMerged(name)}</li>}
      {st.state === "closed" && <li className="font-medium text-attention">{t.closed(name)}</li>}
      {st.head_moved && <li className="font-medium text-attention">{t.headMoved}</li>}
      <li>
        {st.mergeable === "clean" && <span className="rounded-sm bg-success-container px-xs text-on-success-container">{t.clean}</span>}
        {st.mergeable === "conflict" && <span className="rounded-sm bg-error-container px-xs font-medium text-on-error-container">{t.conflict}</span>}
        {st.mergeable === "computing" && <span className="text-muted">{t.computing}</span>}
      </li>
      {st.ci === "forbidden" && <li className="text-muted">{t.ciForbidden}</li>}
      {st.ci === "none" && <li className="text-muted">{t.ciNone}</li>}
      {st.ci === "ok" && (
        <li>
          <p className={count("failure") > 0 ? "font-medium text-error" : undefined}>
            {t.ciSummary(count("success"), count("failure"), count("running"))}
          </p>
          {listed.length > 0 && (
            <ul className="mt-xs flex flex-col">
              {listed.map((c, i) => {
                const href = webPage(c.url);
                return (
                  <li key={`${i}-${c.name}`} className="flex min-h-11 items-center gap-xs break-all">
                    <span className={c.status === "failure" ? "shrink-0 text-error" : "shrink-0 text-muted"}>{t.check[c.status]}</span>
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
 *
 * When the forge refuses the merge for a conflict, the inbox service merges it once the agent has fixed the conflict
 * (ADR-048). The toast says so, and the button reads "自動マージ待ち" ("Auto-merge pending") and cannot be pressed: while autoMerge (the
 * listing's auto_merge) says the inbox service waits, and from the answer for the commit it was pressed on.
 *
 * With ask set, the dialog opens by itself, as when the app's merge button opened the page; onClosed is called
 * whenever the dialog closes.
 */
export function MergeButton({
  issueRef,
  number,
  sha,
  url,
  autoMerge = false,
  ask = false,
  onClosed,
}: {
  issueRef: string;
  number: number;
  sha: string;
  url?: string;
  autoMerge?: boolean;
  ask?: boolean;
  onClosed?: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const backOut = useRef<HTMLButtonElement>(null);
  const id = useId();
  const [sending, setSending] = useState(false);
  const [read, setRead] = useState<Read | null>(null);
  // reads counts the openings: a reading that comes back after the dialog closed, or opened again, is dropped.
  const reads = useRef(0);
  // later is the commit whose merge this page was told waits for the fix; a new done post names another commit.
  const [later, setLater] = useState<string | null>(null);
  const { view, markPending } = useInbox();
  const t = useMessages(messages);
  const merging = view?.some((it) => it.ref === issueRef && it.pending === "merge") ?? false;
  const name = changeName(issueRef, number);
  const waiting = autoMerge || later === sha;
  // later only bridges the time until the list says the merge waits; from then on the list alone says so, so a
  // cancelled or ended merge gives the button back.
  useEffect(() => {
    if (autoMerge) setLater(null);
  }, [autoMerge]);

  useEffect(() => {
    if (ask && !dialog.current?.open) open();
  }, [ask]);

  // The dialog would put the focus on its first link, the change request's; a merge cannot be undone, so the focus
  // starts on backing out instead. React's autoFocus focuses on mount, while the dialog is closed, so it is moved here.
  function open() {
    const n = ++reads.current;
    setRead({ kind: "reading" });
    dialog.current?.showModal();
    backOut.current?.focus();
    mergeStatus(issueRef, sha).then(
      (status) => reads.current === n && setRead({ kind: "read", status }),
      (err: unknown) => reads.current === n && setRead({ kind: "failed", err }),
    );
  }

  function closed() {
    reads.current++;
    setRead(null);
  }

  async function confirm() {
    setSending(true);
    try {
      const res = await merge(issueRef, sha);
      if (res.autoMerge) {
        setLater(sha);
        toast.info(t.mergeLater(name), { description: t.mergeLaterHint });
        return;
      }
      markPending(issueRef, "merge");
      toast.success(t.merged(name), { description: t.closesHint });
    } catch (err) {
      const f = failure(t, name, err);
      toast.error(f.message, { description: f.description });
    } finally {
      setSending(false);
      dialog.current?.close();
    }
  }

  return (
    <>
      <Button
        type="button"
        variant="secondary"
        onClick={open}
        disabled={sending || merging || waiting}
        aria-label={waiting ? t.autoMergePendingLabel(name) : t.mergeLabel(name)}
      >
        {sending && <Spinner />}
        {waiting ? t.autoMergePending : t.merge}
      </Button>
      <Dialog
        ref={dialog}
        aria-labelledby={`${id}-heading`}
        onCancel={(e) => sending && e.preventDefault()}
        onClose={() => {
          closed();
          onClosed?.();
        }}
      >
        <div className="flex flex-col gap-md">
          <h2 id={`${id}-heading`} className="text-headline-md font-semibold">
            {t.askMerge(name)}
          </h2>
          <p className="break-words">{t.mergeExplained(name)}</p>
          {url && (
            <a href={url} target="_blank" rel="noreferrer" className="inline-flex min-h-11 items-center self-start break-all font-mono text-mono-sm underline">
              {url}
            </a>
          )}
          {read && (
            <section aria-label={t.statusLabel(name)} aria-live="polite" className="border-t border-outline pt-md">
              <ChangeStatus name={name} read={read} />
            </section>
          )}
          <div className="flex flex-wrap justify-end gap-sm">
            <Button ref={backOut} type="button" variant="secondary" onClick={() => dialog.current?.close()} disabled={sending}>
              {t.cancel}
            </Button>
            <Button type="button" onClick={() => void confirm()} disabled={sending}>
              {sending && <Spinner />}
              {sending ? t.merging : t.confirmMerge}
            </Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}

/** AutoMergeBadge marks a done issue whose merge the inbox service waits to make once the agent has fixed a conflict. */
export function AutoMergeBadge() {
  const t = useMessages(messages);
  return (
    <Badge tone="neutral" data-auto-merge="">
      {t.autoMergePending}
    </Badge>
  );
}

/**
 * CancelAutoMergeButton cancels the merge the inbox service waits to make on a done issue, after asking in a dialog.
 * The outcome is told in a toast; the badge goes when the listing no longer names the merge.
 */
export function CancelAutoMergeButton({ issueRef, number }: { issueRef: string; number: number }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const backOut = useRef<HTMLButtonElement>(null);
  const id = useId();
  const [sending, setSending] = useState(false);
  const t = useMessages(messages);
  const name = changeName(issueRef, number);

  async function confirm() {
    setSending(true);
    try {
      await cancelAutoMerge(issueRef);
      toast.success(t.cancelled(name));
    } catch (err) {
      if (err instanceof ApiError && err.code === "not_found") {
        toast.error(t.notPending(name), { description: t.notPendingHint });
      } else if (err instanceof ApiError && err.code === "unavailable") {
        toast.error(t.unreachable);
      } else {
        toast.error(t.cancelFailed(name), { description: errorText(err) });
      }
    } finally {
      setSending(false);
      dialog.current?.close();
    }
  }

  function ask() {
    dialog.current?.showModal();
    backOut.current?.focus();
  }

  return (
    <>
      <Button type="button" variant="secondary" onClick={ask} disabled={sending}>
        {sending && <Spinner />}
        {t.cancelAutoMerge}
      </Button>
      <Dialog ref={dialog} aria-labelledby={`${id}-heading`} onCancel={(e) => sending && e.preventDefault()}>
        <div className="flex flex-col gap-md">
          <h2 id={`${id}-heading`} className="text-headline-md font-semibold">
            {t.askCancel(name)}
          </h2>
          <p className="break-words">{t.cancelExplained(name)}</p>
          <div className="flex flex-wrap justify-end gap-sm">
            <Button ref={backOut} type="button" variant="secondary" onClick={() => dialog.current?.close()} disabled={sending}>
              {t.keep}
            </Button>
            <Button type="button" onClick={() => void confirm()} disabled={sending}>
              {sending && <Spinner />}
              {sending ? t.cancelling : t.confirmCancel}
            </Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
