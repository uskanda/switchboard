import { Alert } from "@/components/ui/alert";
import { defineMessages, useLang, useMessages } from "@/lib/i18n";
import { useInbox } from "@/lib/inbox";
import { cn } from "@/lib/utils";

const messages = defineMessages({
  ja: {
    /** forge is a forge the listing could not read, with the forge's own message. */
    forge: (forge: string, message: string) => `${forge}を読めませんでした：${message}`,
    /** limited is a forge that used up its API rate limit, read again at the time given. */
    limited: (forge: string, at: string) => `${forge}のAPI制限中です。${at}に読み直します`,
  },
  en: {
    forge: (forge, message) => `Could not read ${forge}: ${message}`,
    limited: (forge, at) => `${forge} API rate limit reached. Reading it again at ${at}`,
  },
});

/**
 * InboxErrors is the banners for what the listing could not read: the inbox service it could not reach, and each
 * forge it could not read. The list and the issue put them under the header, across both panes, so a narrow screen
 * showing one issue shows them too. A forge that used up its API rate limit says when it is read again, in the
 * browser's time zone. With nothing to say it draws nothing.
 */
export function InboxErrors({ className }: { className?: string }) {
  const { inbox, error } = useInbox();
  const t = useMessages(messages);
  const lang = useLang();
  const forges = inbox?.errors ?? [];
  if (!error && forges.length === 0) return null;
  const time = new Intl.DateTimeFormat(lang, { hour: "2-digit", minute: "2-digit", hourCycle: "h23" });
  return (
    <div className={cn("flex flex-col gap-sm", className)}>
      {error && <Alert>{error}</Alert>}
      {forges.map((e) => (
        <Alert key={e.forge}>
          {e.retry_at ? t.limited(e.forge, time.format(new Date(e.retry_at))) : t.forge(e.forge, e.message)}
        </Alert>
      ))}
    </div>
  );
}
