import { Route, Routes } from "react-router";
import { Toaster } from "@/components/ui/toaster";
import { InboxProvider } from "@/lib/inbox";
import { InboxLayout } from "@/pages/InboxLayout";
import { SettingsPage } from "@/pages/SettingsPage";

/**
 * AppRoutes is the whole web inbox under a router: the list and one issue share a layout; the settings stand apart.
 * The toasts sit beside the routes, so every screen shows them.
 */
export function AppRoutes() {
  return (
    <InboxProvider>
      <Routes>
        <Route path="/" element={<InboxLayout />} />
        <Route path="/issue/:ref" element={<InboxLayout />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Routes>
      <Toaster />
    </InboxProvider>
  );
}
