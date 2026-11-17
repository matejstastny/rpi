import { Plus } from "lucide-react";
import { useState } from "react";
import { ended, type Now } from "@/lib/clock";
import { cn } from "@/lib/cn";
import { dayName, dayNumber, monthName } from "@/lib/dates";
import type { Category, Day, Feed, Item } from "@/lib/types";
import { EventChip } from "./event-chip";
import { ItemCard } from "./item-card";
import { Button } from "./ui";

export function DayColumn({
    day,
    categories,
    feeds,
    now,
    dragging,
    onAdd,
    onMove,
    onComplete,
    onDrop,
    onReopen,
    onEdit,
    onDragStart,
    onDragEnd
}: {
    day: Day;
    categories: Category[];
    feeds: Feed[];
    now: Now;
    dragging: string | null;
    onAdd: (date: string) => void;
    onMove: (id: string, date: string) => void;
    onComplete: (item: Item, rect: DOMRect) => void;
    onDrop: (item: Item) => void;
    onReopen: (item: Item) => void;
    onEdit: (item: Item) => void;
    onDragStart: (id: string) => void;
    onDragEnd: () => void;
}) {
    const [over, setOver] = useState(false);

    // the day's count is about work: an event is not something you get through
    const tasks = day.items.filter((i) => i.kind !== "event");
    const done = tasks.filter((i) => i.status === "done").length;
    const cleared = tasks.length > 0 && tasks.every((i) => i.status !== "open");
    const byID = new Map(categories.map((c) => [c.id, c]));
    const byFeed = new Map(feeds.map((f) => [f.id, f]));

    return (
        <section
            onDragOver={(e) => {
                if (!dragging) return;
                e.preventDefault();
                e.dataTransfer.dropEffect = "move";
                setOver(true);
            }}
            onDragLeave={() => setOver(false)}
            onDrop={(e) => {
                e.preventDefault();
                setOver(false);
                const id = e.dataTransfer.getData("text/plain");
                if (id) onMove(id, day.date);
            }}
            className={cn(
                "group/day flex min-h-56 flex-col rounded-2xl border transition-colors duration-200",
                day.today
                    ? "border-accent/40 bg-surface shadow-[0_0_0_1px_rgb(155_107_201/0.12),0_18px_40px_-26px_rgb(155_107_201/0.9)]"
                    : "border-line/80 bg-surface/40",
                day.past && !day.today && "opacity-65 hover:opacity-100",
                over && "border-accent/70 bg-accent/5"
            )}
        >
            <header
                className={cn(
                    "flex items-baseline justify-between gap-2 border-b px-3 py-2.5",
                    day.today ? "border-accent/20" : "border-line/70"
                )}
            >
                <div className="flex items-baseline gap-2">
                    <span
                        className={cn(
                            "text-[0.68rem] font-semibold tracking-[0.14em] uppercase",
                            day.today ? "text-accent-soft" : "text-dim"
                        )}
                    >
                        {dayName(day.date)}
                    </span>
                    <span
                        className={cn(
                            "font-mono text-sm",
                            day.today ? "font-semibold text-bright" : "text-soft"
                        )}
                    >
                        {dayNumber(day.date)}
                    </span>
                    {dayNumber(day.date) === 1 && (
                        <span className="text-[0.6rem] text-muted uppercase">
                            {monthName(day.date)}
                        </span>
                    )}
                </div>

                {tasks.length > 0 && (
                    <span
                        className={cn(
                            "font-mono text-[0.66rem]",
                            cleared ? "text-green" : "text-muted"
                        )}
                    >
                        {done}/{tasks.length}
                    </span>
                )}
            </header>

            <div className="flex flex-1 flex-col gap-1.5 p-2">
                {day.events.map((event, n) => (
                    <EventChip
                        key={`${event.feed}-${event.title}-${n}`}
                        event={event}
                        over={ended(now, day, event.start, event.end)}
                    />
                ))}

                {day.items.map((item) => (
                    <ItemCard
                        key={item.id}
                        item={item}
                        category={item.category ? byID.get(item.category) : undefined}
                        feed={item.feed ? byFeed.get(item.feed) : undefined}
                        over={ended(now, day, item.start, item.end)}
                        onComplete={(rect) => onComplete(item, rect)}
                        onDrop={() => onDrop(item)}
                        onReopen={() => onReopen(item)}
                        onEdit={() => onEdit(item)}
                        onDragStart={() => onDragStart(item.id)}
                        onDragEnd={onDragEnd}
                    />
                ))}

                {!day.planned && (
                    <p className="px-1 py-3 text-center text-[0.7rem] text-muted/70">nothing yet</p>
                )}
            </div>

            <Button
                variant="ghost"
                size="sm"
                onClick={() => onAdd(day.date)}
                className="m-2 mt-0 justify-start gap-1.5 text-muted opacity-0 transition-opacity group-hover/day:opacity-100 focus-visible:opacity-100 max-lg:opacity-100"
            >
                <Plus className="size-3.5" />
                add
            </Button>
        </section>
    );
}
