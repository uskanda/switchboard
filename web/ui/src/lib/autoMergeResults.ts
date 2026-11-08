import { useEffect, useRef } from "react";
import { autoMergeReason, changeName, type Inbox } from "@/lib/api";
import { currentLang, defineMessages } from "@/lib/i18n";
import { toast } from "@/lib/toast";

/** messages is the text of the toasts that tell a finished automatic merge. */
const messages = defineMessages({
  ja: {
    merged: (name: string) => `${name}を自動でマージしました`,
    closesHint: "issueは、PR/MRの Closes でcloseされると一覧から消えます。",
    stopped: (name: string) => `${name}の自動マージをやめました`,
  },
  en: {
    merged: (name) => `${name} was merged automatically`,
    closesHint: 'The issue leaves the list once "Closes" in the pull or merge request closes it.',
    stopped: (name) => `Auto-merge of ${name} was stopped`,
  },
});

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
 * morning tells what happened overnight, and only that; keys the listing no longer names are forgotten. It is read
 * again on each listing, so a result another tab told is not told again here.
 */
export function useAutoMergeResults(inbox: Inbox | null) {
  const told = useRef<Set<string>>(new Set());
  useEffect(() => {
    const results = inbox?.auto_merge_results;
    if (!inbox) return;
    // The toasts are told from this effect, outside the drawing, in the language shown when the listing came.
    const lang = currentLang();
    const t = messages[lang];
    told.current = new Set([...told.current, ...loadTold()]);
    const listed = new Set<string>();
    for (const r of results ?? []) {
      const key = `${r.ref}@${r.at}`;
      listed.add(key);
      if (told.current.has(key)) continue;
      told.current.add(key);
      const name = changeName(r.ref, r.number);
      if (r.outcome === "merged") {
        toast.success(t.merged(name), { description: t.closesHint });
      } else if (r.reason !== "user") {
        toast.error(t.stopped(name), { description: autoMergeReason(lang, r.reason) });
      }
    }
    told.current = new Set([...told.current].filter((k) => listed.has(k)));
    saveTold(told.current);
  }, [inbox]);
}
