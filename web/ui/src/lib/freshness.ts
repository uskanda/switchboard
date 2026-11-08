import type { Lang } from "@/lib/i18n";

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

/** The short English names of the months, so a date reads the same whatever the browser's locale is. */
export const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** monthDay is the month and the day of t: "11/1" in Japanese, "Nov 1" in English. */
export const monthDay = (lang: Lang, t: Date) =>
  lang === "ja" ? `${t.getMonth() + 1}/${t.getDate()}` : `${MONTHS[t.getMonth()]} ${t.getDate()}`;

/** hourMinute is the time of t to the minute: "9:05" in Japanese, "09:05" in English. */
export const hourMinute = (lang: Lang, t: Date) => `${lang === "ja" ? t.getHours() : two(t.getHours())}:${two(t.getMinutes())}`;

/** clockText is "HH:MM" for a time today, and the date before it on another day: "11/1 14:32", "Nov 1, 14:32". */
export function clockText(lang: Lang, t: Date, now: Date): string {
  const clock = `${two(t.getHours())}:${two(t.getMinutes())}`;
  if (sameDay(t, now)) return clock;
  return lang === "ja" ? `${monthDay(lang, t)} ${clock}` : `${monthDay(lang, t)}, ${clock}`;
}

/** fullText is the date and the time to the second, for a tooltip. */
export function fullText(lang: Lang, t: Date): string {
  const time = `${two(t.getHours())}:${two(t.getMinutes())}:${two(t.getSeconds())}`;
  return lang === "ja"
    ? `${t.getFullYear()}/${t.getMonth() + 1}/${t.getDate()} ${time}`
    : `${monthDay(lang, t)}, ${t.getFullYear()}, ${time}`;
}

/** dateTimeText is the date and the time to the minute, for a comment: "2026/11/02 9:05", "Nov 2, 2026, 09:05". */
export function dateTimeText(lang: Lang, t: Date): string {
  return lang === "ja"
    ? `${t.getFullYear()}/${two(t.getMonth() + 1)}/${two(t.getDate())} ${t.getHours()}:${two(t.getMinutes())}`
    : `${monthDay(lang, t)}, ${t.getFullYear()}, ${two(t.getHours())}:${two(t.getMinutes())}`;
}
