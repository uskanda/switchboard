import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { saveMarks } from "@/lib/pending";
import { toast } from "@/lib/toast";

/** AuthStatus is GET /api/auth: whether people must log in, who is logged in, and the GitLab they log in on. */
export type AuthStatus = { enabled: boolean; user?: string; gitlab?: string; admin?: boolean };

/** The event a call or the event stream dispatches on window when the inbox service answers that no one is logged in. */
export const unauthenticatedEvent = "switchboard:unauthenticated";

/** loggedOut tells every part of the page that the session ended; the gate then shows the login page. */
export function loggedOut() {
  window.dispatchEvent(new Event(unauthenticatedEvent));
}

/** Auth is the login state of the page; known is false until GET /api/auth has answered. */
type Auth = { status: AuthStatus; known: boolean; logout: () => Promise<void> };

const AuthContext = createContext<Auth>({ status: { enabled: false }, known: true, logout: async () => {} });

/** useAuth gives the login state of the page: off, or who is logged in. */
export const useAuth = () => useContext(AuthContext);

/**
 * LoginLanding is /login, where a failed login lands, once someone is logged in or the login is off: the list. Until
 * the status is known it shows nothing, so the reason of a failed login stays in the URL for the login page.
 */
export function LoginLanding({ to }: { to: ReactNode }) {
  return useAuth().known ? <>{to}</> : null;
}

/**
 * readStatus reads GET /api/auth. A web inbox or an inbox service without the login answers it with an error: then
 * the login is off, as it was before it existed.
 */
async function readStatus(): Promise<AuthStatus> {
  try {
    const res = await fetch("/api/auth");
    if (!res.ok) return { enabled: false };
    return (await res.json()) as AuthStatus;
  } catch {
    return { enabled: false };
  }
}

/**
 * AuthGate shows the login page instead of its children when the login is on and no one is logged in, and turns to
 * it when a call or the stream says the session ended. Until GET /api/auth answers, it shows the children: the inbox
 * service and switchboard-web refuse every call without a session, so nothing of the inbox is shown meanwhile.
 */
export function AuthGate({ login, children }: { login: (status: AuthStatus) => ReactNode; children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus | null>(null);
  useEffect(() => {
    let live = true;
    let gone = false;
    // A session that ended while the status was being read outlives the status: no one is logged in. The inbox
    // service answers unauthenticated only with the login on, so the login is on even if its status could not be read.
    readStatus().then((s) => live && setStatus(gone ? { ...s, enabled: true, user: undefined } : s));
    // The marks of what this tab asked (lib/pending.ts) belong to the one logged in: the next one must not see them.
    const ended = () => {
      gone = true;
      saveMarks([]);
      setStatus((s) => ({ ...s, enabled: true, user: undefined }));
    };
    window.addEventListener(unauthenticatedEvent, ended);
    return () => {
      live = false;
      window.removeEventListener(unauthenticatedEvent, ended);
    };
  }, []);
  // A logout the inbox service did not take leaves the session as it was: the page stays logged in and says so.
  const logout = useCallback(async () => {
    const res = await fetch("/api/logout", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }).catch(() => null);
    if (!res?.ok) {
      toast.error("ログアウトできませんでした", { description: "受信箱のサービス（switchboard inbox serve）に届いているか確かめて、もう一度押してください。" });
      return;
    }
    saveMarks([]);
    setStatus((s) => (s ? { ...s, user: undefined } : s));
  }, []);
  if (status?.enabled && !status.user) return <>{login(status)}</>;
  return (
    <AuthContext.Provider value={{ status: status ?? { enabled: false }, known: status !== null, logout }}>{children}</AuthContext.Provider>
  );
}
