import { Alert } from "@/components/ui/alert";
import { defineMessages, useMessages } from "@/lib/i18n";
import { useInbox } from "@/lib/inbox";
import { cn } from "@/lib/utils";

const messages = defineMessages({
  ja: {
    /** forge is a forge the listing could not read, with the forge's own message. */
    forge: (forge: string, message: string) => `${forge}を読めませんでした：${message}`,
  },
  en: {
    forge: (forge, message) => `Could not read ${forge}: ${message}`,
  },
});

/**
 * InboxErrors is the banners for what the listing could not read: the inbox service it could not reach, and each
 * forge it could not read. The list and the issue put them under the header, across both panes, so a narrow screen
 * showing one issue shows them too. With nothing to say it draws nothing.
 */
export function InboxErrors({ className }: { className?: string }) {
  const { inbox, error } = useInbox();
  const t = useMessages(messages);
  const forges = inbox?.errors ?? [];
  if (!error && forges.length === 0) return null;
  return (
    <div className={cn("flex flex-col gap-sm", className)}>
      {error && <Alert>{error}</Alert>}
      {forges.map((e) => (
        <Alert key={e.forge}>{t.forge(e.forge, e.message)}</Alert>
      ))}
    </div>
  );
}
