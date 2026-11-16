/** dates are plain YYYY-MM-DD strings everywhere, the same as the api, so a
 *  day never shifts under a timezone on the way to the screen */

export function toISO(d: Date): string {
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function fromISO(iso: string): Date {
    const [y = 0, m = 1, d = 1] = iso.split("-").map(Number);
    return new Date(y, m - 1, d);
}

export function shift(iso: string, days: number): string {
    const d = fromISO(iso);
    d.setDate(d.getDate() + days);
    return toISO(d);
}

export function mondayOf(d: Date): string {
    const copy = new Date(d);
    copy.setDate(copy.getDate() - ((copy.getDay() + 6) % 7));
    return toISO(copy);
}

const weekday = new Intl.DateTimeFormat(undefined, { weekday: "short" });
const weekdayLong = new Intl.DateTimeFormat(undefined, { weekday: "long" });
const month = new Intl.DateTimeFormat(undefined, { month: "short" });
const monthLong = new Intl.DateTimeFormat(undefined, { month: "long" });

export const dayName = (iso: string) => weekday.format(fromISO(iso));
export const dayNameLong = (iso: string) => weekdayLong.format(fromISO(iso));
export const dayNumber = (iso: string) => fromISO(iso).getDate();
export const monthName = (iso: string) => month.format(fromISO(iso));

/** the header's week label, collapsing the month when both ends share one */
export function spanLabel(start: string, end: string): string {
    const a = fromISO(start);
    const b = fromISO(end);
    const sameMonth = a.getMonth() === b.getMonth();
    const left = `${monthLong.format(a)} ${a.getDate()}`;
    const right = sameMonth ? `${b.getDate()}` : `${monthLong.format(b)} ${b.getDate()}`;
    return `${left} – ${right}`;
}

/** how far the shown week is from the one happening now */
export function weekOffset(start: string, today: string): number {
    const a = fromISO(start).getTime();
    const b = fromISO(mondayOf(fromISO(today))).getTime();
    return Math.round((a - b) / (7 * 24 * 60 * 60 * 1000));
}

export const weekdayInitials = ["M", "T", "W", "T", "F", "S", "S"];
/** monday-first positions mapped onto the api's sunday-first weekday numbers */
export const weekdayOrder = [1, 2, 3, 4, 5, 6, 0];

/** parseClock reads what people actually type into a time field and gives
 *  back HH:MM, or null when it is not a time at all. everything the planner
 *  shows is 24 hour, so a stray am/pm is understood and then thrown away.
 *
 *    13    → 13:00      1330  → 13:30
 *    430   → 04:30      4.30  → 04:30
 *    9     → 09:00      4pm   → 16:00
 */
export function parseClock(raw: string): string | null {
    let text = raw.trim().toLowerCase().replace(/\s+/g, "");
    if (text === "") return "";

    let shift = 0;
    const pm = /(p|pm)$/.test(text);
    const am = /(a|am)$/.test(text);
    if (pm || am) {
        text = text.replace(/(a|p|am|pm)$/, "");
        shift = pm ? 12 : 0;
    }

    let hours: number;
    let minutes = 0;

    const split = text.match(/^(\d{1,2})[:.,h](\d{1,2})$/);
    if (split) {
        hours = Number(split[1]);
        minutes = Number(split[2]);
    } else if (/^\d{1,4}$/.test(text)) {
        // one or two digits is an hour, three or four is hours and minutes
        if (text.length <= 2) {
            hours = Number(text);
        } else {
            hours = Number(text.slice(0, text.length - 2));
            minutes = Number(text.slice(-2));
        }
    } else if (/^(\d{1,2})h$/.test(text)) {
        hours = Number(text.slice(0, -1));
    } else {
        return null;
    }

    if (shift) hours = hours === 12 ? 12 : hours + 12;
    if (am && hours === 12) hours = 0;

    if (!Number.isInteger(hours) || !Number.isInteger(minutes)) return null;
    if (hours > 23 || minutes > 59) return null;

    return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}`;
}
