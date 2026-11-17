import { useEffect, useState } from "react";
import { toISO } from "./dates";
import type { Day } from "./types";

/** the wall clock as the rest of the app spells dates and times: YYYY-MM-DD
 *  and HH:MM, so comparing against an item is a string compare */
export type Now = { date: string; time: string };

function read(): Now {
    const d = new Date();
    const pad = (n: number) => String(n).padStart(2, "0");
    return { date: toISO(d), time: `${pad(d.getHours())}:${pad(d.getMinutes())}` };
}

function untilNextMinute(): number {
    const d = new Date();
    return 60_000 - (d.getSeconds() * 1000 + d.getMilliseconds()) + 50;
}

/** useNow ticks once a minute, on the minute, so the week crosses things off
 *  as the day goes instead of waiting for a reload */
export function useNow(): Now {
    const [now, setNow] = useState(read);

    useEffect(() => {
        let timer = 0;
        const tick = () => {
            setNow((current) => {
                const next = read();
                return next.date === current.date && next.time === current.time ? current : next;
            });
            timer = window.setTimeout(tick, untilNextMinute());
        };
        timer = window.setTimeout(tick, untilNextMinute());
        return () => window.clearTimeout(timer);
    }, []);

    return now;
}

/** ended says whether something that happens at a time already has. Which
 *  day it is belongs to the server, which draws the week in the planner's
 *  timezone, so the column's own flags decide that and the browser clock only
 *  decides how far into today it is. An all-day thing lasts the whole day,
 *  and anything running past midnight is not over until the day turns. */
export function ended(now: Now, day: Day, start?: string, end?: string): boolean {
    if (day.past || now.date > day.date) return true;
    if (!day.today && now.date !== day.date) return false;
    if (end) {
        if (start && end < start) return false;
        return now.time >= end;
    }
    return start ? now.time >= start : false;
}
