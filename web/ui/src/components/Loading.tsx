import { Spinner } from "@/components/ui/spinner";
import { useMessages } from "@/lib/i18n";
import { common } from "@/lib/i18n/common";

/** Loading says that something is being read, in place of what is not there yet. */
export function Loading() {
  const { loading } = useMessages(common);
  return (
    <p role="status" aria-label={loading} className="flex items-center gap-sm text-muted">
      <Spinner />
      {loading}
    </p>
  );
}
