import { useRef, type CSSProperties, type KeyboardEvent, type PointerEvent } from "react";
import { Link, useParams } from "react-router";
import { Header } from "@/components/Header";
import { InboxList } from "@/components/InboxList";
import { IssuePane } from "@/components/IssuePane";
import { IssueRef } from "@/components/IssueRef";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Card } from "@/components/ui/card";
import { issuePath, oldestWaiting, timeText, type Inbox } from "@/lib/api";
import { LIST_WIDTH_DEFAULT, LIST_WIDTH_MAX, LIST_WIDTH_MIN, LIST_WIDTH_STEP, useListWidth } from "@/lib/paneWidth";
import { useInbox } from "@/lib/inbox";
import { usePageTitle } from "@/lib/title";
import { cn } from "@/lib/utils";

/**
 * PaneBorder is the border between the panes, which the user drags (or moves with the arrow keys) to change the list
 * pane's width. A double click puts it back at the default.
 */
function PaneBorder({ width, setWidth }: { width: number; setWidth: (w: number) => void }) {
  const drag = useRef<{ x: number; width: number } | null>(null);

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    e.preventDefault();
    e.currentTarget.setPointerCapture?.(e.pointerId);
    drag.current = { x: e.clientX, width };
  };
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!drag.current) return;
    setWidth(drag.current.width + e.clientX - drag.current.x);
  };
  const onPointerEnd = (e: PointerEvent<HTMLDivElement>) => {
    drag.current = null;
    e.currentTarget.releasePointerCapture?.(e.pointerId);
  };
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const next = { ArrowLeft: width - LIST_WIDTH_STEP, ArrowRight: width + LIST_WIDTH_STEP, Home: LIST_WIDTH_MIN, End: LIST_WIDTH_MAX }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    setWidth(next);
  };

  return (
    <div
      role="separator"
      aria-label="一覧の幅"
      aria-orientation="vertical"
      aria-valuenow={width}
      aria-valuemin={LIST_WIDTH_MIN}
      aria-valuemax={LIST_WIDTH_MAX}
      tabIndex={0}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerEnd}
      onPointerCancel={onPointerEnd}
      onKeyDown={onKeyDown}
      onDoubleClick={() => setWidth(LIST_WIDTH_DEFAULT)}
      className="absolute inset-y-0 left-[calc(var(--list-width)-4px)] z-10 hidden w-2 cursor-col-resize touch-none select-none outline-none after:absolute after:inset-y-0 after:left-[3px] after:w-0.5 hover:after:bg-outline focus-visible:after:bg-primary active:after:bg-primary lg:block"
    />
  );
}

/**
 * NothingChosen fills the right pane while no issue is chosen: a card that leads to the issue that has waited longest,
 * or, while nothing waits, a line that asks to choose one from the list.
 */
function NothingChosen({ inbox }: { inbox: Inbox | null }) {
  const it = oldestWaiting(inbox?.items ?? []);
  if (!it) return <p className="pt-xl text-muted">左の一覧からissueを選んでください</p>;
  return (
    <section aria-labelledby="oldest-waiting" className="flex flex-col gap-sm pt-xl">
      <h2 id="oldest-waiting" className="text-label-md font-semibold text-muted">
        最も古い待ち
      </h2>
      <Link to={issuePath(it.ref)} className="block rounded-md focus-visible:outline-2 focus-visible:outline-primary">
        <Card className="flex flex-col gap-xs">
          <IssueRef value={it.ref} className="self-start" />
          <span className="line-clamp-2 font-medium break-words" title={it.title}>
            {it.title}
          </span>
          <span className="flex flex-wrap items-center gap-sm text-label-md text-muted">
            <ReasonBadge reason={it.reason} />
            <span>{timeText("waiting", it.waiting_since)}</span>
          </span>
        </Card>
      </Link>
    </section>
  );
}

/**
 * InboxLayout draws / and /issue/:ref. From 1024px it puts the list in a left pane and the selected issue in the
 * right one, with a border the user drags to resize the list; below that, / shows the list alone and /issue/:ref the
 * issue alone. With no issue chosen, the right pane leads to the one that has waited longest (NothingChosen).
 */
export function InboxLayout() {
  const ref = useParams().ref;
  const [width, setWidth] = useListWidth();
  const { inbox } = useInbox();
  usePageTitle(ref && (inbox?.items.find((it) => it.ref === ref)?.title ?? ref));
  return (
    <div className="flex min-h-dvh flex-col lg:h-dvh">
      <Header />
      <div
        style={{ "--list-width": `${width}px` } as CSSProperties}
        className="flex-1 lg:relative lg:grid lg:min-h-0 lg:grid-cols-[var(--list-width)_minmax(0,1fr)]"
      >
        <nav aria-label="一覧" className={cn("min-w-0 px-lg py-xl lg:overflow-y-auto lg:border-r lg:border-outline", ref && "hidden lg:block")}>
          <InboxList selected={ref} />
        </nav>
        <PaneBorder width={width} setWidth={setWidth} />
        {/* No padding on top: the issue's pinned head brings its own, so nothing shows above it once scrolled. */}
        <main className={cn("px-lg pb-xl lg:overflow-y-auto", !ref && "hidden lg:block")}>

          <div className="mx-auto max-w-[720px]">
            {ref ? <IssuePane issueRef={ref} /> : <NothingChosen inbox={inbox} />}
          </div>
        </main>
      </div>
    </div>
  );
}
