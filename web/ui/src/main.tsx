import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Route, Routes } from "react-router";
import "./index.css";
import { InboxPage } from "./pages/InboxPage";
import { IssuePage } from "./pages/IssuePage";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<InboxPage />} />
        <Route path="/issue/:ref" element={<IssuePage />} />
      </Routes>
    </BrowserRouter>
  </StrictMode>,
);
