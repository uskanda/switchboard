import { Spinner } from "@/components/ui/spinner";

/** Loading says that something is being read, in place of what is not there yet. */
export function Loading() {
  return (
    <p role="status" aria-label="読み込んでいます" className="flex items-center gap-sm text-muted">
      <Spinner />
      読み込んでいます
    </p>
  );
}
