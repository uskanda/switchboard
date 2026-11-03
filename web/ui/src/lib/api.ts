// The web inbox's back end (switchboard-web), which passes these on to the inbox service. The types follow
// package inboxapi.

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

/** autoMergeReason says why the inbox service stopped waiting to merge. */
export function autoMergeReason(reason: string | undefined): string {
  switch (reason) {
    case "closed":
      return "issueがcloseされました。";
    case "waiting":
      return "issueが返答待ちになりました。衝突の直しが上限を超えたときもこうなります。";
    case "instruction":
      return "人の指示で実行が始まりました。変更を見てからマージしてください。";
    case "not_fix":
      return "直す実行でない投稿で、コミットが変わりました。変更を見てからマージしてください。";
    case "changed":
      return "完了の投稿が、別のPR/MRを指しています。";
    case "change_closed":
      return "PR/MRが閉じられたか、マージ済みです。";
    case "pushed":
      return "エージェントのものでないコミットが、PR/MRに足されました。";
    case "forbidden":
      return "受信箱のトークンに、マージする権限がありません。";
    case "user":
      return "取り消しました。";
  }
  return reason ?? "";
}

export const isRunning = (it: Item) => it.state === "running";
export const isQueued = (it: Item) => it.state === "queued";
export const isDone = (it: Item) => it.state === "done";
/** isWaiting is an issue waiting on a reply: not running, queued or done. */
export const isWaiting = (it: Item) => !isRunning(it) && !isQueued(it) && !isDone(it);

/**
 * progressText is what a run is doing as shown on the list: the agent's last words, after the first separator ("｜"
 * or "|") of the progress line, without the time and the tools before it. Without words yet ("?", nothing after the
 * separator, no separator or no progress comment at all), it says the run is working.
 */
export function progressText(progress: string | undefined): string {
  const line = progress ?? "";
  const at = line.search(/[｜|]/);
  const words = at < 0 ? "" : line.slice(at + 1).trim();
  return words && words !== "?" ? words : "作業しています";
}

/** plainText is the start of a question as one plain line: the bold and code marks of its Markdown left out. */
export const plainText = (s: string) => s.replace(/\*\*|__|`/g, "").replace(/^#+\s+/, "");

/**
 * queueText says which limit a queued issue waits on: a full slot of the host or the project, or a pause of the host
 * on a usage limit or the API quota, with the time the pause ends (local time) when there is one.
 */
export function queueText(reason: string, until?: string | null): string {
  const pause: Record<string, string> = { limit: "利用上限に達しています", quota: "APIのクオータに達しています" };
  if (reason in pause) {
    if (!until) return pause[reason];
    const d = new Date(until);
    const hm = `${d.getHours()}:${String(d.getMinutes()).padStart(2, "0")}`;
    return `${pause[reason]}（${d.getMonth() + 1}/${d.getDate()} ${hm}まで）`;
  }
  return reason === "project-max-parallel" ? "プロジェクトの上限に達しています" : "端末の上限に達しています";
}

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
  plan_first: boolean;
  cloned: boolean;
  missing_labels: string[];
  labels_error?: string;
};

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

export const addProject = (ref: string, planFirst: boolean) =>
  call<AddedProject>("/api/projects", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ref, plan_first: planFirst }),
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
  /** plan_first is whether the project's ready label starts with a plan; an older inbox service leaves it out. */
  items: { ref: string; host: string; plan_first?: boolean }[];
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

/** age is how long ago since was, in Japanese, or "?" without a time. */
export function age(since: string | null, now: Date = new Date()): string {
  if (!since) return "?";
  const min = Math.max(0, Math.floor((now.getTime() - new Date(since).getTime()) / 60000));
  if (min < 1) return "1分未満";
  if (min < 60) return `${min}分前`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}時間前`;
  return `${Math.floor(h / 24)}日前`;
}

/** elapsed is how long a run has gone on since it started, in Japanese, or "?" without a time. */
export function elapsed(since: string | null | undefined, now: Date = new Date()): string {
  if (!since) return "?";
  const min = Math.max(0, Math.floor((now.getTime() - new Date(since).getTime()) / 60000));
  if (min < 1) return "1分未満";
  if (min < 60) return `${min}分`;
  return `${Math.floor(min / 60)}時間${min % 60}分`;
}

/** TimeState is a state whose time is shown: since when it runs, waits, queues or is done. */
export type TimeState = "running" | "waiting" | "queued" | "done";

/**
 * timeText is the time of a state in words that say what it is: "2分経過", "5分前から待ち", "3分前から順番待ち",
 * "1時間前に完了". A time under a minute says how long the state has lasted ("待ちになって1分未満"); without a time it
 * is "?".
 */
export function timeText(state: TimeState, since: string | null | undefined, now: Date = new Date()): string {
  if (!since) return "?";
  const under = Math.floor((now.getTime() - new Date(since).getTime()) / 60000) < 1;
  switch (state) {
    case "running":
      return under ? "始めて1分未満" : `${elapsed(since, now)}経過`;
    case "waiting":
      return under ? "待ちになって1分未満" : `${age(since, now)}から待ち`;
    case "queued":
      return under ? "順番待ちになって1分未満" : `${age(since, now)}から順番待ち`;
    case "done":
      return under ? "完了して1分未満" : `${age(since, now)}に完了`;
  }
}

/** oldestWaiting is the waiting item that started waiting first; one without a time comes last. */
export function oldestWaiting(items: Item[]): Item | undefined {
  const time = (it: Item) => (it.waiting_since ? new Date(it.waiting_since).getTime() : Infinity);
  return items.filter(isWaiting).reduce<Item | undefined>((best, it) => (!best || time(it) < time(best) ? it : best), undefined);
}

/** attention tells the reasons that wait on a decision from the ones that report a failure. */
export const isFailure = (reason: string) => ["error", "permission", "push", "ci", "conflict"].includes(reason);
