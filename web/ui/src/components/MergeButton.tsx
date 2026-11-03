import { useEffect, useId, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { ApiError, cancelAutoMerge, changeName, merge } from "@/lib/api";
import { useInbox } from "@/lib/inbox";
import { toast } from "@/lib/toast";

export { changeName };

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

/**
 * MergeButton merges the pull or merge request of a done issue: the one its latest done post names, at the commit
 * the page showed. It asks first in a dialog, since a merge cannot be undone; the outcome is told in a toast. The
 * issue itself is closed by the change request's "Closes #N", and the list drops it on its next read (ADR-043).
 * Until then the issue is marked as being merged (lib/pending.ts), and the button cannot be pressed.
 *
 * When the forge refuses the merge for a conflict, the inbox service merges it once the agent has fixed the conflict
 * (ADR-048). The toast says so, and the button reads "自動マージ待ち" and cannot be pressed: while autoMerge (the
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
  // later is the commit whose merge this page was told waits for the fix; a new done post names another commit.
  const [later, setLater] = useState<string | null>(null);
  const { view, markPending } = useInbox();
  const merging = view?.some((it) => it.ref === issueRef && it.pending === "merge") ?? false;
  const name = changeName(issueRef, number);
  const waiting = autoMerge || later === sha;

  useEffect(() => {
    if (ask && !dialog.current?.open) open();
  }, [ask]);

  async function confirm() {
    setSending(true);
    try {
      const res = await merge(issueRef, sha);
      if (res.autoMerge) {
        setLater(sha);
        toast.info(`${name}は衝突しているため、エージェントが直したあと自動でマージします`, {
          description: "やめるときは、issueを開いて「自動マージを取り消す」を押してください。",
        });
        return;
      }
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

  // The dialog would put the focus on its first link, the change request's; a merge cannot be undone, so the focus
  // starts on backing out instead. React's autoFocus focuses on mount, while the dialog is closed, so it is moved here.
  function open() {
    dialog.current?.showModal();
    backOut.current?.focus();
  }

  return (
    <>
      <Button
        type="button"
        variant="secondary"
        onClick={open}
        disabled={sending || merging || waiting}
        aria-label={waiting ? `${name}は自動マージ待ち` : `${name}をマージ`}
      >
        {sending && <Spinner />}
        {waiting ? "自動マージ待ち" : "マージ"}
      </Button>
      <Dialog ref={dialog} aria-labelledby={`${id}-heading`} onCancel={(e) => sending && e.preventDefault()} onClose={() => onClosed?.()}>
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

/** AutoMergeBadge marks a done issue whose merge the inbox service waits to make once the agent has fixed a conflict. */
export function AutoMergeBadge() {
  return (
    <Badge tone="neutral" data-auto-merge="">
      自動マージ待ち
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
  const name = changeName(issueRef, number);

  async function confirm() {
    setSending(true);
    try {
      await cancelAutoMerge(issueRef);
      toast.success(`${name}の自動マージを取り消しました`);
    } catch (err) {
      if (err instanceof ApiError && err.code === "not_found") {
        toast.error(`${name}の自動マージは、もう待っていません。`, { description: "マージされたか、やめた後です。開き直してください。" });
      } else if (err instanceof ApiError && err.code === "unavailable") {
        toast.error("受信箱のサービス（switchboard inbox serve）に届きません。");
      } else {
        toast.error(`${name}の自動マージを取り消せませんでした。`, { description: err instanceof Error ? err.message : String(err) });
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
        自動マージを取り消す
      </Button>
      <Dialog ref={dialog} aria-labelledby={`${id}-heading`} onCancel={(e) => sending && e.preventDefault()}>
        <div className="flex flex-col gap-md">
          <h2 id={`${id}-heading`} className="text-headline-md font-semibold">
            {name}の自動マージを取り消しますか
          </h2>
          <p className="break-words">エージェントが衝突を直しても、{name}はマージされません。マージするときは、もう一度マージを押してください。</p>
          <div className="flex flex-wrap justify-end gap-sm">
            <Button ref={backOut} type="button" variant="secondary" onClick={() => dialog.current?.close()} disabled={sending}>
              やめる
            </Button>
            <Button type="button" onClick={() => void confirm()} disabled={sending}>
              {sending && <Spinner />}
              {sending ? "取り消しています" : "取り消す"}
            </Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
