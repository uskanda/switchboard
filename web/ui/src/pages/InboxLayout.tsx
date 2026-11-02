import { useEffect, useRef, type CSSProperties, type KeyboardEvent, type PointerEvent } from "react";
import { useNavigate, useParams } from "react-router";
import { Header } from "@/components/Header";
import { InboxList } from "@/components/InboxList";
import { IssuePane } from "@/components/IssuePane";
import { isWaiting, issuePath } from "@/lib/api";
import { LIST_WIDTH_DEFAULT, LIST_WIDTH_MAX, LIST_WIDTH_MIN, LIST_WIDTH_STEP, useListWidth } from "@/lib/paneWidth";
import { useInbox } from "@/lib/inbox";
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
 * InboxLayout draws / and /issue/:ref. From 1024px it puts the list in a left pane and the selected issue in the
 * right one, with a border the user drags to resize the list; below that, / shows the list alone and /issue/:ref the
 * issue alone.
 * On either, j and k open the wait below and above the open issue (lib/waits.ts), and r moves the focus to the reply;
 * from 1024px, a line under the list tells these keys.
 */
export function InboxLayout() {
  const ref = useParams().ref;
  const [width, setWidth] = useListWidth();
  const { inbox } = useInbox();
  usePageTitle(ref && (inbox?.items.find((it) => it.ref === ref)?.title ?? ref));
  const navigate = useNavigate();
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
      const to = stepWait(inbox?.items.filter(isWaiting) ?? [], ref, e.key === "j" ? 1 : -1);
      if (!to) return;
      e.preventDefault();
      void navigate(issuePath(to.ref));
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [inbox, ref, navigate]);
  return (
    <div className="flex min-h-dvh flex-col lg:h-dvh">
      <Header />
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
            {ref ? <IssuePane issueRef={ref} /> : <p className="pt-xl text-muted">左の一覧からissueを選んでください</p>}
          </div>
        </main>
      </div>
    </div>
  );
}
