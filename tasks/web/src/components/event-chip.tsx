import { CalendarDays, Check, MapPin } from "lucide-react";
import { cn } from "@/lib/cn";
import type { CalendarEvent } from "@/lib/types";

/** an imported event: something happening to you rather than something you
 *  chose, so it is never completed by hand. Once it is over the day crosses
 *  it off for you, the same as a finished task. */
export function EventChip({ event, over }: { event: CalendarEvent; over: boolean }) {
    return (
        <div
            className={cn(
                "animate-rise group/event relative overflow-hidden rounded-xl border px-2.5 py-1.5 pl-3",
                over ? "border-green/20 bg-green/5" : "border-line/70 bg-surface/60"
            )}
            title={`${event.title} — ${event.name}`}
        >
            <span
                aria-hidden
                className={cn(
                    "absolute inset-y-1.5 left-0 w-[3px] rounded-full",
                    over ? "opacity-40" : "opacity-70"
                )}
                style={{ background: over ? "var(--color-green)" : event.color }}
            />
            <div className="flex items-baseline gap-1.5">
                {over ? (
                    <Check className="size-3 shrink-0 translate-y-0.5 text-green/70" />
                ) : (
                    <CalendarDays className="size-3 shrink-0 translate-y-0.5 text-muted" />
                )}
                <span
                    className={cn(
                        "min-w-0 flex-1 truncate text-[0.76rem]",
                        over ? "text-dim line-through decoration-green/50" : "text-soft"
                    )}
                >
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
