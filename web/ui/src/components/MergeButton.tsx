import { useEffect, useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { ApiError, merge } from "@/lib/api";
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

/**
 * MergeButton merges the pull or merge request of a done issue: the one its latest done post names, at the commit
 * the page showed. It asks first in a dialog, since a merge cannot be undone; the outcome is told in a toast. The
 * issue itself is closed by the change request's "Closes #N", and the list drops it on its next read (ADR-043).
 * With ask set, the dialog opens by itself, as when the app's merge button opened the page; onClosed is called
 * whenever the dialog closes.
 */
export function MergeButton({
  issueRef,
  number,
  sha,
  url,
  ask = false,
  onClosed,
}: {
  issueRef: string;
  number: number;
  sha: string;
  url?: string;
  ask?: boolean;
  onClosed?: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const id = useId();
  const [sending, setSending] = useState(false);
  const name = changeName(issueRef, number);

  useEffect(() => {
    if (ask && !dialog.current?.open) dialog.current?.showModal();
  }, [ask]);

  async function confirm() {
    setSending(true);
    try {
      await merge(issueRef, sha);
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
      <Button type="button" variant="secondary" onClick={() => dialog.current?.showModal()} disabled={sending} aria-label={`${name}をマージ`}>
        {sending && <Spinner />}
        マージ
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
            <Button type="button" variant="secondary" onClick={() => dialog.current?.close()} disabled={sending}>
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
