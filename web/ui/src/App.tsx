import { useMatch } from "react-router";
import { InboxList } from "@/components/InboxList";
import { IssuePane } from "@/components/IssuePane";
import { Alert } from "@/components/ui/alert";
import { ApiError } from "@/lib/api";
import { useLiveInbox, type Connection } from "@/lib/live";
import { cn } from "@/lib/utils";

function failure(err: ApiError): string {
  if (err.code === "unavailable") {
    return "受信箱のサービス（switchboard inbox serve）に届きません。サービスが動いているかを確かめてください。";
  }
  return `一覧を取れませんでした：${err.message}`;
}

const connectionText: Record<Connection, string> = {
  connecting: "つないでいます",
  live: "自動更新中",
  lost: "更新が途切れました。つなぎ直しています",
};

/**
 * App is the whole page: a header, the list on the left, and the picked issue on the right. A narrow screen shows
 * one of the two: the list at /, the issue at /issue/:ref.
 */
export function App() {
  const ref = useMatch("/issue/:ref")?.params.ref ?? null;
  const { inbox, error, connection } = useLiveInbox();
  const item = inbox?.items.find((it) => it.ref === ref);

  return (
    <div className="flex min-h-dvh flex-col md:h-dvh">
      <header className="flex flex-wrap items-center justify-between gap-sm border-b border-outline px-lg py-md">
        <h1 className="text-headline-md font-semibold">受信箱</h1>
        <p role="status" aria-label="更新の状態" className="flex items-center gap-xs text-label-md text-muted">
          <span
            aria-hidden
            className={cn("size-2 rounded-full", connection === "live" ? "bg-muted" : "border border-muted")}
          />
          {connectionText[connection]}
        </p>
      </header>
      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <section
          aria-label="一覧"
          className={cn(
            "flex-col gap-sm p-lg md:flex md:w-[360px] md:shrink-0 md:overflow-y-auto md:border-r md:border-outline",
            ref ? "hidden" : "flex",
          )}
        >
          {error && <Alert>{failure(error)}</Alert>}
          {inbox?.errors.map((e) => (
            <Alert key={e.forge}>
              {e.forge}を読めませんでした：{e.message}
            </Alert>
          ))}
          {inbox && <InboxList items={inbox.items} selected={ref} />}
        </section>
        <main className={cn("min-w-0 flex-1 p-lg md:block md:overflow-y-auto", ref ? "block" : "hidden")}>
          {ref ? (
            <IssuePane key={ref} issueRef={ref} item={item} listed={inbox !== null} />
          ) : (
            <p className="text-muted">issueを選ぶと、ここに質問の全文が出ます</p>
          )}
        </main>
      </div>
    </div>
  );
}
