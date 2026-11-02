import { useEffect } from "react";
import { isWaiting } from "@/lib/api";
import { useInbox } from "@/lib/inbox";

const app = "SwitchBoard Inbox";

/**
 * usePageTitle names the tab after the screen (name, when given) and leads with the count of waiting issues while
 * there are any, so a tab in the background shows that something waits.
 */
export function usePageTitle(name?: string) {
  const { inbox } = useInbox();
  const waiting = inbox?.items.filter(isWaiting).length ?? 0;
  useEffect(() => {
    document.title = `${waiting > 0 ? `(${waiting}) ` : ""}${name ? `${name} - ` : ""}${app}`;
  }, [name, waiting]);
}
