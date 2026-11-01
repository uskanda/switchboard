import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
import { forgetIssues } from "@/lib/issueCache";

afterEach(() => {
  cleanup();
  forgetIssues();
});
