// The web inbox's back end (switchboard-web), which passes these on to the inbox service. The types follow
// package inboxapi.

export type Item = {
  /** The row number; only a waiting issue has one. */
  n?: number;
  /** waiting or running; a back end older than the running list leaves it out, and its items all wait. */
  state?: "waiting" | "running";
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
  /** The forge's update time and count of people's comments (GitLab's; 0 on GitHub): a move in either means the issue changed. */
  updated_at?: string | null;
  notes?: number;
};

export const isRunning = (it: Item) => it.state === "running";

export type Inbox = { items: Item[]; errors: { forge: string; message: string }[]; fetched_at: string };

export type Detail = {
  ref: string;
  title: string;
  url: string;
  waiting: boolean;
  host: string;
  state: string;
  reason: string;
  since: string | null;
  post: string;
  /** The word whose reply approves a plan, such as /ok. */
  approve_word?: string;
  /** The issue's own author, time and body, and all its comments, oldest first. A back end older than the thread leaves them out. */
  author?: string;
  created_at?: string | null;
  body?: string;
  comments?: Comment[];
};

/**
 * Comment is one comment of an issue. agent is set on a post of the agent, whose kind is then the state of its
 * marker (running for the claim of a run, progress, waiting, done); a person's comment is as written.
 */
export type Comment = {
  author: string;
  created_at: string;
  body: string;
  agent: boolean;
  kind?: "running" | "progress" | "waiting" | "done" | string;
};

/** watchKey is what moves when an issue on the list changes: a page showing the issue reads it again on a move. */
export const watchKey = (it: Item) =>
  JSON.stringify([it.updated_at ?? null, it.notes ?? 0, it.state ?? "waiting", it.reason, it.progress ?? null]);

/** One file of the plan runs' additional instructions; target "" is the one for every project. */
export type Instruction = { target: string; content: string };

/** Instructions are read on host, the machine of the inbox service. */
export type Instructions = { host: string; items: Instruction[] };

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

export const saveInstructions = (target: string, content: string) =>
  call<void>("/api/instructions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ target, content }),
  });

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

/** attention tells the reasons that wait on a decision from the ones that report a failure. */
export const isFailure = (reason: string) => reason === "error" || reason === "permission";
