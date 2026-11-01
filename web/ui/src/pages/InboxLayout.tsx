import { useParams } from "react-router";
import { Header } from "@/components/Header";
import { InboxList } from "@/components/InboxList";
import { IssuePane } from "@/components/IssuePane";
import { cn } from "@/lib/utils";

/**
 * InboxLayout draws / and /issue/:ref. From 1024px it puts the list in a left pane and the selected issue in the
 * right one; below that, / shows the list alone and /issue/:ref the issue alone.
 */
export function InboxLayout() {
  const ref = useParams().ref;
  return (
    <div className="flex min-h-dvh flex-col lg:h-dvh">
      <Header />
      <div className="flex-1 lg:grid lg:min-h-0 lg:grid-cols-[360px_minmax(0,1fr)]">
        <nav aria-label="一覧" className={cn("px-lg py-xl lg:overflow-y-auto lg:border-r lg:border-outline", ref && "hidden lg:block")}>
          <InboxList selected={ref} />
        </nav>
        <main className={cn("px-lg py-xl lg:overflow-y-auto", !ref && "hidden lg:block")}>
          <div className="mx-auto max-w-[720px]">
            {ref ? <IssuePane issueRef={ref} /> : <p className="text-muted">左の一覧からissueを選んでください</p>}
          </div>
        </main>
      </div>
    </div>
  );
}
