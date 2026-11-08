import { defineMessages } from "@/lib/i18n";

/**
 * common is the text more than one part of the inbox shows: the states of an issue, the error of an inbox service
 * that does not answer, the lead of an error the server explains, and the words of the buttons every dialog has.
 */
export const common = defineMessages({
  ja: {
    /** The states of an issue, as a section of the list and a count name them. */
    state: { waiting: "返答待ち", running: "実行中", queued: "順番待ち", done: "完了" },
    unreachable: "受信箱のサービス（switchboard inbox serve）に届きません。",
    /** failed leads the message of an error the server sent, which stays in its own language. */
    failed: (detail: string) => `失敗しました：${detail}`,
    cancel: "やめる",
    loading: "読み込んでいます",
  },
  en: {
    state: { waiting: "Waiting", running: "Running", queued: "Queued", done: "Done" },
    unreachable: "Cannot reach the inbox service (switchboard inbox serve).",
    failed: (detail) => `Failed: ${detail}`,
    cancel: "Cancel",
    loading: "Loading",
  },
});
