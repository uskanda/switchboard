import { useEffect, useRef } from "react";
import { autoMergeReason, changeName, type Inbox } from "@/lib/api";
import { toast } from "@/lib/toast";

/** storageKey is where the finished automatic merges already told are remembered, as "ref@at". */
const storageKey = "switchboard.autoMergeTold";

function loadTold(): Set<string> {
  try {
    const v: unknown = JSON.parse(window.localStorage.getItem(storageKey) ?? "[]");
    return new Set(Array.isArray(v) ? v.filter((k): k is string => typeof k === "string") : []);
  } catch {
    return new Set();
  }
}

function saveTold(told: Set<string>) {
  try {
    window.localStorage.setItem(storageKey, JSON.stringify([...told]));
  } catch {
    // A browser without storage tells a result again on the next visit; nothing else is lost.
  }
}

/**
 * useAutoMergeResults tells, once each, the merges the inbox service waited to make and finished (the listing's
 * auto_merge_results): a merge in a success toast, a merge it stopped waiting for in an error toast with the reason.
 * A cancel the person made is not told again. What was told is remembered in localStorage, so a page opened the next
 * morning tells what happened overnight, and only that; keys the listing no longer names are forgotten.
 */
export function useAutoMergeResults(inbox: Inbox | null) {
  const told = useRef<Set<string> | null>(null);
  useEffect(() => {
    const results = inbox?.auto_merge_results;
    if (!inbox) return;
    told.current ??= loadTold();
    const listed = new Set<string>();
    for (const r of results ?? []) {
      const key = `${r.ref}@${r.at}`;
      listed.add(key);
      if (told.current.has(key)) continue;
      told.current.add(key);
      const name = changeName(r.ref, r.number);
      if (r.outcome === "merged") {
        toast.success(`${name}を自動でマージしました`, { description: "issueは、PR/MRの Closes でcloseされると一覧から消えます。" });
      } else if (r.reason !== "user") {
        toast.error(`${name}の自動マージをやめました`, { description: autoMergeReason(r.reason) });
      }
    }
    told.current = new Set([...told.current].filter((k) => listed.has(k)));
    saveTold(told.current);
  }, [inbox]);
}
