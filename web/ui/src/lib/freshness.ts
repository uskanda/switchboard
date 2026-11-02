/**
 * How old the list is while the automatic updates are not live. The inbox service sends a listing only when it
 * changes, so fetched_at alone is the time of the last change; up to the moment the stream was cut, the list was
 * known to be current. The list is therefore as of the later of the two.
 */

const parse = (s: string | null): Date | null => {
  if (!s) return null;
  const d = new Date(s);
  return Number.isNaN(d.getTime()) ? null : d;
};

/** asOf is the time the list was last known to be current: the later of fetched_at and liveUntil. */
export function asOf(fetchedAt: string, liveUntil: string | null): Date | null {
  const a = parse(fetchedAt);
  const b = parse(liveUntil);
  if (!a || !b) return a ?? b;
  return a > b ? a : b;
}

const two = (n: number) => String(n).padStart(2, "0");
const sameDay = (a: Date, b: Date) =>
  a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();

/** clockText is "HH:MM" for a time today, and "M/D HH:MM" for one on another day. */
export function clockText(t: Date, now: Date): string {
  const clock = `${two(t.getHours())}:${two(t.getMinutes())}`;
  return sameDay(t, now) ? clock : `${t.getMonth() + 1}/${t.getDate()} ${clock}`;
}

/** fullText is the date and the time to the second, for a tooltip. */
export function fullText(t: Date): string {
  return `${t.getFullYear()}/${t.getMonth() + 1}/${t.getDate()} ${two(t.getHours())}:${two(t.getMinutes())}:${two(t.getSeconds())}`;
}
