import { Link } from "react-router";
import { ReasonBadge } from "@/components/ReasonBadge";
import { age, issuePath, type Item } from "@/lib/api";
import { cn } from "@/lib/utils";

/** InboxList is the left pane's list of waiting issues, oldest wait first as the inbox service sends them. */
export function InboxList({ items, selected }: { items: Item[]; selected: string | null }) {
  if (items.length === 0) return <p className="text-muted">待ちのissueはありません</p>;
  return (
    <nav aria-label="待ちのissue">
      <ul className="flex flex-col gap-sm">
        {items.map((it) => {
          const current = it.ref === selected;
          return (
            <li key={it.ref}>
              <Link
                to={issuePath(it.ref)}
                aria-current={current ? "page" : undefined}
                className={cn(
                  "flex flex-col gap-xs rounded-md border bg-surface-raised p-lg focus-visible:outline-2 focus-visible:outline-primary",
                  current ? "border-primary" : "border-outline",
                )}
              >
                <span className="font-mono text-mono-sm break-all">{it.ref}</span>
                <span className="font-medium break-words">{it.title}</span>
                <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
                  <ReasonBadge reason={it.reason} />
                  <span>{it.host}</span>
                  <span>{age(it.waiting_since)}</span>
                </span>
                <span className="line-clamp-2 text-label-md text-muted break-words">{it.question}</span>
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
