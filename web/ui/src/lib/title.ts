import { useEffect } from "react";
import { isWaiting } from "@/lib/api";
import { useInbox } from "@/lib/inbox";

const app = "SwitchBoard Inbox";

/**
 * usePageTitle names the tab after the screen (name, when given) and leads with the count of waiting issues while
 * there are any, so a tab in the background shows that something waits. An issue replied to from this tab is counted
 * where it is shown, under running (lib/pending.ts).
 */
export function usePageTitle(name?: string) {
  const { view } = useInbox();
  const waiting = view?.filter(isWaiting).length ?? 0;
  useEffect(() => {
    document.title = `${waiting > 0 ? `(${waiting}) ` : ""}${name ? `${name} - ` : ""}${app}`;
  }, [name, waiting]);
}
