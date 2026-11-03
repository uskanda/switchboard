import { useCallback, useState } from "react";

/** Where the browser remembers whether the list is grouped by project. */
export const GROUP_BY_PROJECT_KEY = "switchboard.groupByProject";

/** readGroupByProject is the stored choice: off only for "false", on when nothing or something else is stored. */
export function readGroupByProject(stored: string | null): boolean {
  return stored !== "false";
}

function load(): boolean {
  try {
    return readGroupByProject(window.localStorage.getItem(GROUP_BY_PROJECT_KEY));
  } catch {
    return true;
  }
}

function save(on: boolean) {
  try {
    window.localStorage.setItem(GROUP_BY_PROJECT_KEY, String(on));
  } catch {
    // A browser without storage keeps the choice for this page only.
  }
}

/** useGroupByProject is whether the list is grouped by project, on at first and remembered in localStorage. */
export function useGroupByProject() {
  const [on, setRaw] = useState(load);
  const setOn = useCallback((next: boolean) => {
    setRaw(next);
    save(next);
  }, []);
  return [on, setOn] as const;
}
