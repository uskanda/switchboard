import { act } from "@testing-library/react";

/**
 * jsdom lays nothing out: it has no scrollIntoView and no IntersectionObserver. These fakes stand in for them. The
 * elements scrolled to are recorded in scrolled, and a test makes an element enter or leave the screen with intersect.
 */
export const scrolled: Element[] = [];

class FakeIntersectionObserver {
  static all = new Set<FakeIntersectionObserver>();
  private targets = new Set<Element>();
  constructor(private readonly callback: IntersectionObserverCallback) {
    FakeIntersectionObserver.all.add(this);
  }
  observe(el: Element) {
    this.targets.add(el);
  }
  unobserve(el: Element) {
    this.targets.delete(el);
  }
  disconnect() {
    this.targets.clear();
    FakeIntersectionObserver.all.delete(this);
  }
  takeRecords() {
    return [];
  }
  fire(el: Element, isIntersecting: boolean) {
    if (!this.targets.has(el)) return;
    const entry = { target: el, isIntersecting, intersectionRatio: isIntersecting ? 1 : 0 } as unknown as IntersectionObserverEntry;
    this.callback([entry], this as unknown as IntersectionObserver);
  }
}

/** intersect tells every observer watching el that it entered (true) or left (false) the screen. */
export function intersect(el: Element, isIntersecting: boolean) {
  act(() => FakeIntersectionObserver.all.forEach((o) => o.fire(el, isIntersecting)));
}

/** installObservers puts the fakes in place of the browser's; resetObservers forgets what the last test did. */
export function installObservers() {
  Element.prototype.scrollIntoView = function (this: Element) {
    scrolled.push(this);
  };
  globalThis.IntersectionObserver = FakeIntersectionObserver as unknown as typeof IntersectionObserver;
}

export function resetObservers() {
  scrolled.length = 0;
  FakeIntersectionObserver.all.clear();
}
