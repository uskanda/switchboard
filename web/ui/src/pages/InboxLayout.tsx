import { useEffect, useLayoutEffect, useRef, type CSSProperties, type KeyboardEvent, type PointerEvent } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { Header } from "@/components/Header";
import { InboxErrors } from "@/components/InboxErrors";
import { InboxList } from "@/components/InboxList";
import { IssuePane } from "@/components/IssuePane";
import { IssueRef } from "@/components/IssueRef";
import { ReasonBadge } from "@/components/ReasonBadge";
import { Card } from "@/components/ui/card";
import { isDone, isQueued, isRunning, isWaiting, issuePath, oldestWaiting, timeText, type Item } from "@/lib/api";
import { keepListScroll, listScroll } from "@/lib/listScroll";
import { LIST_WIDTH_DEFAULT, LIST_WIDTH_MAX, LIST_WIDTH_MIN, LIST_WIDTH_STEP, useListWidth } from "@/lib/paneWidth";
import { useInbox } from "@/lib/inbox";
import { useMinute } from "@/lib/now";
import { usePageTitle } from "@/lib/title";
import { cn } from "@/lib/utils";
import { ignoreKey, stepWait } from "@/lib/waits";

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
 * Summary is the inbox at a glance, for the right pane while no issue is chosen (issue #183): how many issues each
 * section of the list has, then how many waits each reason has, most first, with the reason's badge.
 */
function Summary({ items }: { items: Item[] }) {
  const sections = [
    { name: "返答待ち", n: items.filter(isWaiting).length },
    { name: "実行中", n: items.filter(isRunning).length },
    { name: "順番待ち", n: items.filter(isQueued).length },
    { name: "完了", n: items.filter(isDone).length },
  ];
  const reasons = new Map<string, number>();
  for (const it of items.filter(isWaiting)) reasons.set(it.reason, (reasons.get(it.reason) ?? 0) + 1);
  const byCount = [...reasons].sort((a, b) => b[1] - a[1]);
  return (
    <section aria-labelledby="inbox-summary" className="flex flex-col gap-sm">
      <h2 id="inbox-summary" className="text-label-md font-semibold text-muted">
        受信箱の概要
      </h2>
      <Card className="flex flex-col gap-md">
        <dl className="grid grid-cols-4 gap-sm">
          {sections.map((s) => (
            <div key={s.name} className="flex flex-col gap-xs">
              <dt className="text-label-md text-muted">{s.name}</dt>
              <dd className="text-headline-md font-semibold">{`${s.n}件`}</dd>
            </div>
          ))}
        </dl>
        {byCount.length > 0 && (
          <ul aria-label="待ちの理由" className="flex flex-col gap-xs border-t border-outline pt-md">
            {byCount.map(([reason, n]) => (
              <li key={reason} className="flex items-center justify-between gap-sm">
                <ReasonBadge reason={reason} />
                <span className="text-label-md text-muted">{`${n}件`}</span>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </section>
  );
}

/**
 * NothingChosen fills the right pane while no issue is chosen: the summary of the inbox, then a card that leads to the
 * issue that has waited longest, or, while nothing waits, a line that asks to choose one from the list.
 */
function NothingChosen({ items }: { items: Item[] | null }) {
  useMinute();
  // The waits as the list shows them: an issue being processed (lib/pending.ts) is not offered.
  const it = oldestWaiting(items ?? []);
  return (
    <div className="flex flex-col gap-xl pt-xl">
      {items && <Summary items={items} />}
      {it ? <OldestWaiting it={it} /> : <p className="text-muted">左の一覧からissueを選んでください</p>}
    </div>
  );
}

/** OldestWaiting is the card that leads to the issue that has waited longest. */
function OldestWaiting({ it }: { it: Item }) {
  return (
    <section aria-labelledby="oldest-waiting" className="flex flex-col gap-sm">
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
 * On either, j and k open the wait below and above the open issue (lib/waits.ts), and r moves the focus to the reply;
 * from 1024px, a line under the list tells these keys.
 * Below 1024px the page itself scrolls: the list comes back scrolled where it was left (lib/listScroll.ts), and an
 * issue starts at its top (IssuePane).
 */
export function InboxLayout() {
  const ref = useParams().ref;
  const [width, setWidth] = useListWidth();
  const { inbox, view } = useInbox();
  usePageTitle(ref && (inbox?.items.find((it) => it.ref === ref)?.title ?? ref));
  const navigate = useNavigate();
  useLayoutEffect(() => {
    if (ref) return;
    window.scrollTo(0, listScroll());
    const keep = () => keepListScroll(window.scrollY);
    window.addEventListener("scroll", keep, { passive: true });
    return () => window.removeEventListener("scroll", keep);
  }, [ref]);
  useEffect(() => {
    const onKeyDown = (e: globalThis.KeyboardEvent) => {
      if (!["j", "k", "r"].includes(e.key) || ignoreKey(e)) return;
      if (e.key === "r") {
        const reply = document.getElementById("reply");
        if (!reply) return;
        // Stopped, or the r would be typed into the field the focus just moved to.
        e.preventDefault();
        reply.focus();
        return;
      }
      // The waits as the list shows them: an issue being processed (lib/pending.ts) is shown under running.
      const to = stepWait(view?.filter(isWaiting) ?? [], ref, e.key === "j" ? 1 : -1);
      if (!to) return;
      e.preventDefault();
      void navigate(issuePath(to.ref));
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [view, ref, navigate]);
  return (
    <div className="flex min-h-dvh flex-col lg:h-dvh">
      <Header />
      <InboxErrors className="px-lg pt-lg" />
      <div
        style={{ "--list-width": `${width}px` } as CSSProperties}
        className="flex-1 lg:relative lg:grid lg:min-h-0 lg:grid-cols-[var(--list-width)_minmax(0,1fr)]"
      >
        <nav aria-label="一覧" className={cn("min-w-0 px-lg py-xl lg:overflow-y-auto lg:border-r lg:border-outline", ref && "hidden lg:block")}>
          <InboxList selected={ref} />
          <p className="mt-lg hidden text-label-md text-muted lg:block">{"j/k：待ちの間を移る　r：返答の欄へ"}</p>
        </nav>
        <PaneBorder width={width} setWidth={setWidth} />
        {/* No padding on top: the issue's pinned head brings its own, so nothing shows above it once scrolled. */}
        <main className={cn("px-lg pb-xl lg:overflow-y-auto", !ref && "hidden lg:block")}>

          <div className="mx-auto max-w-[720px]">
            {ref ? <IssuePane issueRef={ref} /> : <NothingChosen items={view} />}
          </div>
        </main>
      </div>
    </div>
  );
}
