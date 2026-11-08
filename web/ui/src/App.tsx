import { Navigate, Route, Routes } from "react-router";
import { Toaster } from "@/components/ui/toaster";
import { AuthGate, LoginLanding } from "@/lib/auth";
import { useDocumentLang } from "@/lib/i18n";
import { InboxProvider } from "@/lib/inbox";
import { InboxLayout } from "@/pages/InboxLayout";
import { LoginPage } from "@/pages/LoginPage";
import { NotFoundPage } from "@/pages/NotFoundPage";
import { SettingsPage } from "@/pages/SettingsPage";

/**
 * AppRoutes is the whole web inbox under a router: the list and one issue share a layout; the settings stand apart,
 * and any other URL gets a page that says it does not exist.
 * The toasts sit beside the routes, so every screen shows them. With inbox.auth set, nothing of the inbox is read
 * until someone has logged in: the gate shows the login page instead (issue #15). /login is where a failed login
 * lands; once logged in, it is the list. The page's lang attribute follows the language shown, on every screen.
 */
export function AppRoutes() {
  useDocumentLang();
  return (
    <AuthGate login={() => <LoginPage />}>
      <InboxProvider>
        <Routes>
          <Route path="/" element={<InboxLayout />} />
          <Route path="/login" element={<LoginLanding to={<Navigate to="/" replace />} />} />
          <Route path="/issue/:ref" element={<InboxLayout />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
        <Toaster />
      </InboxProvider>
    </AuthGate>
  );
}
