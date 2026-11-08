// The web inbox's back end (switchboard-web), which passes these on to the inbox service. The types follow
// package inboxapi.

import { loggedOut } from "@/lib/auth";
import { hourMinute, monthDay } from "@/lib/freshness";
import { defineMessages, type Lang } from "@/lib/i18n";

export type Item = {
  /** The row number; only a waiting or done issue has one. */
  n?: number;
  /** waiting, done, running or queued; a back end older than the running list leaves it out, and its items all wait. */
  state?: "waiting" | "done" | "running" | "queued";
  ref: string;
  project: string;
  number: number;
  title: string;
  url: string;
  host: string;
  waiting_since: string | null;
  reason: string;
  question: string;
  running_since?: string | null;
  /** The last line of the run's progress comment, "?" without one. */
  progress?: string;
  /** When the issue was queued for a free slot; reason then names the limit reached. */
  queued_since?: string | null;
  /** When the pause of the host ends, for an issue queued on one (reason limit or quota). */
  queued_until?: string | null;
  /** When the agent finished an issue still open (it waits to be closed), and the pull or merge request it names. */
  done_since?: string | null;
  change_url?: string;
  /** The pull or merge request and the commit the done post's marker names; the merge is offered only with both. */
  change_number?: number;
  change_sha?: string;
  /** The forge's update time and count of people's comments (GitLab's; 0 on GitHub): a move in either means the issue changed. */
  updated_at?: string | null;
  notes?: number;
  /** Set while the inbox service waits to merge the change request once the agent has fixed its conflict. */
  auto_merge?: AutoMerge;
};

/** AutoMerge is a merge the inbox service waits to make: the commit the merge was pressed on, and when. */
export type AutoMerge = { sha: string; since: string };

/** AutoMergeResult is a merge the inbox service waited to make and finished within the last day. */
export type AutoMergeResult = {
  ref: string;
  number: number;
  outcome: "merged" | "cancelled";
  reason?: string;
  at: string;
};

/** changeName is how the forge names the change request: "PR #9" on GitHub, "MR !9" on GitLab. */
export function changeName(ref: string, number: number) {
  return ref.startsWith("gl:") ? `MR !${number}` : `PR #${number}`;
}

/** autoMergeReason says why the inbox service stopped waiting to merge; a reason it does not know shows as it came. */
export function autoMergeReason(lang: Lang, reason: string | undefined): string {
  const known = autoMergeReasons[lang] as Record<string, string>;
  return (reason !== undefined && known[reason]) || (reason ?? "");
}

const autoMergeReasons = defineMessages({
  ja: {
    closed: "issueがcloseされました。",
    waiting: "issueが返答待ちになりました。衝突の直しが上限を超えたときもこうなります。",
    instruction: "人の指示で実行が始まりました。変更を見てからマージしてください。",
    not_fix: "直す実行でない投稿で、コミットが変わりました。変更を見てからマージしてください。",
    changed: "完了の投稿が、別のPR/MRを指しています。",
    change_closed: "PR/MRが閉じられたか、マージ済みです。",
    pushed: "エージェントのものでないコミットが、PR/MRに足されました。",
    forbidden: "受信箱のトークンに、マージする権限がありません。",
    user: "取り消しました。",
    restarted: "受信箱のサービスが再起動し、マージを頼んだ人が分からなくなりました。もう一度マージを押してください。",
  },
  en: {
    closed: "The issue was closed.",
    waiting: "The issue is waiting for a reply. This also happens when the conflict fixes exceed their limit.",
    instruction: "A person's instruction started a run. Review the changes, then merge.",
    not_fix: "The commits changed outside a fix run. Review the changes, then merge.",
    changed: "The done post points to another pull or merge request.",
    change_closed: "The pull or merge request was closed or is already merged.",
    pushed: "A commit that is not the agent's was added to the pull or merge request.",
    forbidden: "The inbox token has no permission to merge.",
    user: "Cancelled.",
    restarted: 'The inbox service restarted and no longer knows who asked for the merge. Press "Merge" again.',
  },
});

export const isRunning = (it: Item) => it.state === "running";
export const isQueued = (it: Item) => it.state === "queued";
export const isDone = (it: Item) => it.state === "done";
/** isWaiting is an issue waiting on a reply: not running, queued or done. */
export const isWaiting = (it: Item) => !isRunning(it) && !isQueued(it) && !isDone(it);

/** ListState is what the list and the issue's own read are compared on: the state alone, not the reason. */
export type ListState = "running" | "queued" | "done" | "waiting";

/** itemState is the state a row of the list shows. */
export const itemState = (it: Item): ListState => (isRunning(it) ? "running" : isQueued(it) ? "queued" : isDone(it) ? "done" : "waiting");

/** detailState is the state an issue's own read shows, in the order IssuePane shows it: a run or a queue goes first. */
export const detailState = (d: Detail): ListState =>
  d.state === "running" ? "running" : d.state === "queued" ? "queued" : d.done ? "done" : "waiting";

/**
 * stateDiffers is whether the issue's own read shows another state than its row on the list, read earlier: the
 * inbox service then reads the list again, and the issue says the list has yet to catch up (issue #82).
 */
export const stateDiffers = (it: Item, d: Detail) => itemState(it) !== detailState(d);

/**
 * progressText is what a run is doing as shown on the list: the agent's last words, after the first separator ("｜"
 * or "|") of the progress line, without the time and the tools before it. Without words yet ("?", nothing after the
 * separator, no separator or no progress comment at all), it says the run is working.
 */
export function progressText(lang: Lang, progress: string | undefined): string {
  const line = progress ?? "";
  const at = line.search(/[｜|]/);
  const words = at < 0 ? "" : line.slice(at + 1).trim();
  return words && words !== "?" ? words : progressWords[lang].working;
}

const progressWords = defineMessages({
  ja: { working: "作業しています" },
  en: { working: "Working" },
});

/** ProgressLine is one line of a run's progress comment: the time, elapsed time and tools, then the agent's words. */
export type ProgressLine = { head: string; words: string };

/**
 * parseProgress reads the lines of a run's progress comment, the list items under its heading, oldest first, each cut
 * at its first separator ("｜" or "|"). A comment with no list item is not read, and gives null.
 */
export function parseProgress(body: string): ProgressLine[] | null {
  const lines = body
    .split("\n")
    .filter((l) => l.startsWith("- "))
    .map((l) => {
      const line = l.slice(2).trim();
      const at = line.search(/[｜|]/);
      return at < 0 ? { head: line, words: "" } : { head: line.slice(0, at).trim(), words: line.slice(at + 1).trim() };
    });
  return lines.length > 0 ? lines : null;
}

/** plainText is the start of a question as one plain line: the bold and code marks of its Markdown left out. */
export const plainText = (s: string) => s.replace(/\*\*|__|`/g, "").replace(/^#+\s+/, "");

/**
 * queueText says which limit a queued issue waits on: a full slot of the host or the project, or a pause of the host
 * on a usage limit or the API quota, with the time the pause ends (local time) when there is one.
 */
export function queueText(lang: Lang, reason: string, until?: string | null): string {
  const t = queueTexts[lang];
  const pause: Record<string, string> = { limit: t.limit, quota: t.quota };
  if (reason in pause) {
    if (!until) return pause[reason];
    const d = new Date(until);
    return t.until(pause[reason], monthDay(lang, d), hourMinute(lang, d));
  }
  return reason === "project-max-parallel" ? t.project : t.host;
}

const queueTexts = defineMessages({
  ja: {
    limit: "利用上限に達しています",
    quota: "APIのクオータに達しています",
    project: "プロジェクトの上限に達しています",
    host: "端末の上限に達しています",
    until: (pause: string, day: string, hm: string) => `${pause}（${day} ${hm}まで）`,
  },
  en: {
    limit: "Usage limit reached",
    quota: "API quota reached",
    project: "Project limit reached",
    host: "Host limit reached",
    until: (pause, day, hm) => `${pause} (until ${day}, ${hm})`,
  },
});

/**
 * Inbox is a listing. relay is whether the inbox service follows the relay's events ("" when it is not set up), and
 * refresh_interval_seconds how often it reads the forges while a page listens; older services send neither.
 */
export type Inbox = {
  items: Item[];
  errors: { forge: string; message: string }[];
  fetched_at: string;
  relay?: "connected" | "disconnected" | "";
  refresh_interval_seconds?: number;
  /** The merges the inbox service waited to make and finished within the last day, oldest first. */
  auto_merge_results?: AutoMergeResult[];
};

export type Detail = {
  ref: string;
  title: string;
  url: string;
  waiting: boolean;
  /** Open and done: it takes a reply as a waiting issue does, until it is closed. A back end older than this leaves it out. */
  done?: boolean;
  host: string;
  state: string;
  reason: string;
  since: string | null;
  post: string;
  /** The word whose reply approves a plan, such as /ok. */
  approve_word?: string;
  /** Set on an open done issue, as on its Item. */
  change_url?: string;
  change_number?: number;
  change_sha?: string;
  auto_merge?: AutoMerge;
  /** The issue's own author, time and body, and all its comments, oldest first. A back end older than the thread leaves them out. */
  author?: string;
  created_at?: string | null;
  body?: string;
  comments?: Comment[];
  /** With the login on, whether the agent takes the logged-in person's reply as an instruction (trusted_authors). */
  trusted?: boolean;
};

/**
 * Comment is one comment of an issue. agent is set on a post of the agent, whose kind is then the state of its
 * marker (running for the claim of a run, progress, queued, waiting, done); a person's comment is as written.
 */
export type Comment = {
  author: string;
  created_at: string;
  body: string;
  agent: boolean;
  kind?: "running" | "progress" | "queued" | "waiting" | "done" | string;
};

/** watchKey is what moves when an issue on the list changes: a page showing the issue reads it again on a move. */
export const watchKey = (it: Item) =>
  JSON.stringify([it.updated_at ?? null, it.notes ?? 0, it.state ?? "waiting", it.reason, it.progress ?? null]);

/**
 * One file of the additional instructions; target "" is the one for every project. kind "plan" is for the plan runs,
 * "run" for the runs that write files, "implement" for the direct runs and the runs of an approved plan only; an inbox
 * service older than kinds leaves it out, which means "plan".
 */
export type InstructionKind = "plan" | "run" | "implement";
export type Instruction = { target: string; kind?: InstructionKind; content: string };

/** Instructions are read on host, the machine of the inbox service. */
export type Instructions = { host: string; items: Instruction[] };

/**
 * Project is one project of the inbox service's machine: whether its clone is at path, and the state labels its
 * repository lacks. labels_error says why the labels could not be read; missing_labels is then empty.
 */
export type Project = {
  ref: string;
  path: string;
  /** plan_first is true when plan_first_mode is always or auto; an older inbox service gives only this. */
  plan_first: boolean;
  plan_first_mode?: PlanFirstMode;
  cloned: boolean;
  missing_labels: string[];
  labels_error?: string;
};

/**
 * PlanFirstMode is how a project's ready label starts: always with a plan that waits for approval, with a plan only
 * when the agent finds something to ask (auto), or never.
 */
export type PlanFirstMode = "always" | "auto" | "never";

/**
 * PlanStart is how a project's ready label starts, as far as the inbox service says: its plan_first_mode, or, from an
 * older service that gives only the boolean, "plan" for true and "never" for false. It is undefined when the service
 * says neither.
 */
export type PlanStart = PlanFirstMode | "plan";

export function planStartOf(p: { plan_first?: boolean; plan_first_mode?: PlanFirstMode }): PlanStart | undefined {
  if (p.plan_first_mode) return p.plan_first_mode;
  if (p.plan_first === undefined) return undefined;
  return p.plan_first ? "plan" : "never";
}

/** Projects are host's; enabled says whether one may be added (inbox.project_root is set). */
export type Projects = { host: string; enabled: boolean; items: Project[] };

/** AddedProject is the answer to an addition; webhook, when set, is the command that registers the relay's webhook. */
export type AddedProject = { project: Project; webhook?: string };

export class ApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    if (res.status === 401 && body.code === "unauthenticated") loggedOut();
    throw new ApiError(body.code ?? "unknown", body.message ?? `${res.status} ${res.statusText}`);
  }
  return (res.status === 204 ? undefined : await res.json()) as T;
}

export const getInbox = () => call<Inbox>("/api/inbox");

export const getIssue = (ref: string) => call<Detail>(`/api/issue?ref=${encodeURIComponent(ref)}`);

export const postReply = (ref: string, text: string) =>
  call<void>("/api/reply", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ref, text }),
  });

export const getInstructions = () => call<Instructions>("/api/instructions");

export const saveInstructions = (target: string, kind: InstructionKind, content: string) =>
  call<void>("/api/instructions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ target, kind, content }),
  });

export const getProjects = () => call<Projects>("/api/projects");

/** addProject sends the mode, and the boolean an older inbox service reads instead (true for always and auto). */
export const addProject = (ref: string, mode: PlanFirstMode) =>
  call<AddedProject>("/api/projects", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ref, plan_first: mode !== "never", plan_first_mode: mode }),
  });

/**
 * The kinds of start label a new issue can carry: the configuration's labels.ready, labels.plan and labels.go, and
 * none, no label, whose label is empty.
 */
export type StartKind = "ready" | "plan" | "go" | "none";

/**
 * IssueTargets are every configured project, each with host, the machine that runs it, and the start labels the
 * configuration names, then none.
 */
export type IssueTargets = {
  /**
   * plan_first_mode is how the project's ready label starts, and plan_first whether it may start with a plan. An older
   * inbox service leaves plan_first_mode out, and one older still both.
   */
  items: { ref: string; host: string; plan_first?: boolean; plan_first_mode?: PlanFirstMode }[];
  starts: { kind: StartKind; label: string }[];
};

/** NewIssue is what a new issue is opened with; the inbox service turns start into the label, or none. */
export type NewIssue = { ref: string; title: string; body: string; start: StartKind };

/** CreatedIssue is the issue opened: its reference and its page on the forge. */
export type CreatedIssue = { ref: string; url: string };

export const getIssueTargets = () => call<IssueTargets>("/api/issue-targets");

export const createIssue = (issue: NewIssue) =>
  call<CreatedIssue>("/api/issues", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(issue),
  });

/**
 * merge marks the pull or merge request of a done issue ready and merges it with the inbox token, only at sha, the
 * commit the page showed. It is refused with stale, not_mergeable or forbidden. autoMerge is set when the forge
 * refused it for a conflict, and the inbox service merges it once the agent has fixed the conflict.
 */
export async function merge(ref: string, sha: string): Promise<{ autoMerge: boolean }> {
  const out = await call<{ code?: string } | undefined>("/api/merge", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ref, sha }),
  });
  return { autoMerge: out?.code === "auto_merge_pending" };
}

/** cancelAutoMerge cancels the merge the inbox service waits to make on the issue; not_found when there is none. */
export const cancelAutoMerge = (ref: string) =>
  call<void>("/api/automerge/cancel", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ref }),
  });

/**
 * getProjectIcons reads the emoji icons of the configuration, by project reference. The other projects' icons are
 * images at /api/project-icon?ref=, which answers 404 when a project has none.
 */
export const getProjectIcons = () => call<Record<string, string>>("/api/project-icons");

/** CheckStatus is one CI check of the commit. name is the forge's data: show it as text only. */
export type CheckStatus = { name: string; status: "success" | "failure" | "running" | "other"; url: string };

/**
 * MergeStatus is where the change request of a done issue stands before the merge (ADR-050): its state on the forge,
 * whether its head moved from the commit the page showed, whether it conflicts (computing while the forge works it
 * out), and the checks of that commit. ci is "forbidden" when the inbox token may not read CI, "none" without checks.
 */
export type MergeStatus = {
  state: "open" | "closed" | "merged";
  head_moved: boolean;
  mergeable: "clean" | "conflict" | "computing";
  ci: "ok" | "none" | "forbidden";
  checks: CheckStatus[];
};

/** mergeStatus reads the status of the change request of a done issue at sha, for the merge dialog. */
export const mergeStatus = (ref: string, sha: string) =>
  call<MergeStatus>(`/api/merge-status?ref=${encodeURIComponent(ref)}&sha=${encodeURIComponent(sha)}`);

/** issuePath is the page of one issue; the ref is one path segment. */
export const issuePath = (ref: string) => `/issue/${encodeURIComponent(ref)}`;

const minutesSince = (since: string, now: Date) =>
  Math.max(0, Math.floor((now.getTime() - new Date(since).getTime()) / 60000));

/** span is a length of time in the CLI's short units, the largest that fits: "5m", "3h", "2d". */
function span(min: number): string {
  if (min < 60) return `${min}m`;
  const h = Math.floor(min / 60);
  return h < 24 ? `${h}h` : `${Math.floor(h / 24)}d`;
}

/** age is how long ago since was ("5分前", "5m ago"), or "?" without a time. */
export function age(lang: Lang, since: string | null, now: Date = new Date()): string {
  if (!since) return "?";
  const min = minutesSince(since, now);
  const t = spanWords[lang];
  if (min < 1) return t.ageUnder;
  if (min < 60) return t.minutesAgo(min);
  const h = Math.floor(min / 60);
  return h < 24 ? t.hoursAgo(h) : t.daysAgo(Math.floor(h / 24));
}

/** elapsed is how long a run has gone on since it started ("1時間3分", "1h 3m"), or "?" without a time. */
export function elapsed(lang: Lang, since: string | null | undefined, now: Date = new Date()): string {
  if (!since) return "?";
  const min = minutesSince(since, now);
  const t = spanWords[lang];
  if (min < 1) return t.elapsedUnder;
  if (min < 60) return t.minutes(min);
  return t.hoursMinutes(Math.floor(min / 60), min % 60);
}

// The words of age and elapsed; in English, the CLI's short units.
const spanWords = defineMessages({
  ja: {
    ageUnder: "1分未満",
    minutesAgo: (n: number) => `${n}分前`,
    hoursAgo: (n: number) => `${n}時間前`,
    daysAgo: (n: number) => `${n}日前`,
    elapsedUnder: "1分未満",
    minutes: (m: number) => `${m}分`,
    hoursMinutes: (h: number, m: number) => `${h}時間${m}分`,
  },
  en: {
    ageUnder: "just now",
    minutesAgo: (n) => `${n}m ago`,
    hoursAgo: (n) => `${n}h ago`,
    daysAgo: (n) => `${n}d ago`,
    elapsedUnder: "under 1m",
    minutes: (m) => `${m}m`,
    hoursMinutes: (h, m) => `${h}h ${m}m`,
  },
});

/** TimeState is a state whose time is shown: since when it runs, waits, queues or is done. */
export type TimeState = "running" | "waiting" | "queued" | "done";

/**
 * timeText is the time of a state in words that say what it is: "2分経過", "5分前から待ち", "3分前から順番待ち",
 * "1時間前に完了"; in English "2m elapsed", "waiting 5m", "queued 3m", "done 1h ago". A time under a minute says how
 * long the state has lasted ("待ちになって1分未満", "waiting under 1m"); without a time it is "?".
 */
export function timeText(lang: Lang, state: TimeState, since: string | null | undefined, now: Date = new Date()): string {
  if (!since) return "?";
  const under = Math.floor((now.getTime() - new Date(since).getTime()) / 60000) < 1;
  const t = timeTexts[lang];
  switch (state) {
    case "running":
      return under ? t.runningUnder : t.running(elapsed(lang, since, now));
    case "waiting":
      return under ? t.waitingUnder : t.waiting(ago(lang, since, now));
    case "queued":
      return under ? t.queuedUnder : t.queued(ago(lang, since, now));
    case "done":
      return under ? t.doneUnder : t.done(age(lang, since, now));
  }
}

/** Ago is how long ago a state began, as an age ("5分前") and as a span ("5m"), so each language says it its own way. */
type Ago = { age: string; span: string };
const ago = (lang: Lang, since: string, now: Date): Ago => ({ age: age(lang, since, now), span: span(minutesSince(since, now)) });

// The words of timeText.
const timeTexts = defineMessages({
  ja: {
    running: (elapsed: string) => `${elapsed}経過`,
    waiting: ({ age }: Ago) => `${age}から待ち`,
    queued: ({ age }: Ago) => `${age}から順番待ち`,
    done: (age: string) => `${age}に完了`,
    runningUnder: "始めて1分未満",
    waitingUnder: "待ちになって1分未満",
    queuedUnder: "順番待ちになって1分未満",
    doneUnder: "完了して1分未満",
  },
  en: {
    running: (elapsed) => `${elapsed} elapsed`,
    waiting: ({ span }) => `waiting ${span}`,
    queued: ({ span }) => `queued ${span}`,
    done: (age) => `done ${age}`,
    runningUnder: "started just now",
    waitingUnder: "waiting under 1m",
    queuedUnder: "queued under 1m",
    doneUnder: "done just now",
  },
});

/** oldestWaiting is the waiting item that started waiting first; one without a time comes last. */
export function oldestWaiting(items: Item[]): Item | undefined {
  const time = (it: Item) => (it.waiting_since ? new Date(it.waiting_since).getTime() : Infinity);
  return items.filter(isWaiting).reduce<Item | undefined>((best, it) => (!best || time(it) < time(best) ? it : best), undefined);
}

/** attention tells the reasons that wait on a decision from the ones that report a failure. */
export const isFailure = (reason: string) => ["error", "permission", "push", "ci", "conflict"].includes(reason);
