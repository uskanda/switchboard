import { Badge } from "@/components/ui/badge";
import { forgeName, splitRef, type Forge } from "@/lib/ref";
import { cn } from "@/lib/utils";

/** ForgeBadge names the forge in words; its color only helps to tell the two apart (DESIGN.md's badge-github and badge-gitlab). */
export function ForgeBadge({ forge }: { forge: Forge }) {
  return <Badge tone={forge}>{forgeName[forge]}</Badge>;
}

/**
 * IssueRef shows a reference of an issue or a project without the forge's prefix, the forge as a badge after it:
 * "o/r#12 [GitHub]". The space between the two keeps them apart in the accessible name ("o/r#12 GitHub").
 */
export function IssueRef({ value, className }: { value: string; className?: string }) {
  const { forge, path } = splitRef(value);
  return (
    <span className={cn("inline-flex flex-wrap items-center gap-xs align-middle", className)}>
      <span className="font-mono text-mono-sm break-all">{path}</span>
      {forge && (
        <>
          {" "}
          <ForgeBadge forge={forge} />
        </>
      )}
    </span>
  );
}
