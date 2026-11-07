import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
import { forgetDrafts } from "@/lib/drafts";
import { forgetInstructionDrafts } from "@/lib/instructionDrafts";
import { forgetIssues } from "@/lib/issueCache";
import { forgetListScroll } from "@/lib/listScroll";
import { forgetProjectIcons } from "@/lib/projectIcons";
import { forgetSent } from "@/lib/sent";
import { dismiss } from "@/lib/toast";
import { installObservers, resetObservers } from "@/test-observers";

// jsdom has no modal dialogs: showModal and close only open and close the dialog, as far as the tests look.
if (typeof HTMLDialogElement.prototype.showModal !== "function") {
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.removeAttribute("open");
    this.dispatchEvent(new Event("close"));
  };
}

installObservers();

// jsdom does not scroll the window; a call to scroll it does nothing, as far as the tests look.
window.scrollTo = (() => {}) as typeof window.scrollTo;

// jsdom has no pointer capture, which sonner takes when a toast is pressed (to swipe it away).
if (typeof Element.prototype.setPointerCapture !== "function") {
  Element.prototype.setPointerCapture = () => {};
  Element.prototype.releasePointerCapture = () => {};
  Element.prototype.hasPointerCapture = () => false;
}

afterEach(() => {
  cleanup();
  forgetIssues();
  forgetProjectIcons();
  forgetDrafts();
  forgetInstructionDrafts();
  forgetSent();
  forgetListScroll();
  resetObservers();
  dismiss();
  // The pending marks (lib/pending.ts) live in the tab's storage, which jsdom keeps across the tests of a file.
  sessionStorage.clear();
});
