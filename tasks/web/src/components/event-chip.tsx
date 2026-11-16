import { CalendarDays, MapPin } from "lucide-react";
import type { CalendarEvent } from "@/lib/types";

/** an imported event: something happening to you rather than something you
 *  chose, so it is shown but never completed */
export function EventChip({ event }: { event: CalendarEvent }) {
    return (
        <div
            className="animate-rise group/event relative overflow-hidden rounded-xl border border-line/70 bg-surface/60 px-2.5 py-1.5 pl-3"
            title={`${event.title} — ${event.name}`}
        >
            <span
                aria-hidden
                className="absolute inset-y-1.5 left-0 w-[3px] rounded-full opacity-70"
                style={{ background: event.color }}
            />
            <div className="flex items-baseline gap-1.5">
                <CalendarDays className="size-3 shrink-0 translate-y-0.5 text-muted" />
                <span className="min-w-0 flex-1 truncate text-[0.76rem] text-soft">
                    {event.title}
                </span>
            </div>
            <div className="mt-0.5 flex items-center gap-2 pl-4.5 text-[0.64rem] text-muted">
                {event.start ? (
                    <span className="font-mono">
                        {event.start}
                        {event.end && `–${event.end}`}
                    </span>
                ) : (
                    <span className="font-mono">all day</span>
                )}
                {event.where && (
                    <span className="inline-flex min-w-0 items-center gap-1">
                        <MapPin className="size-2.5 shrink-0" />
                        <span className="truncate">{event.where}</span>
                    </span>
                )}
            </div>
        </div>
    );
}
