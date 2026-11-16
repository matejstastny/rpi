import {
    Ban,
    CalendarClock,
    CalendarDays,
    Check,
    Clock,
    Repeat,
    RotateCcw,
    Undo2
} from "lucide-react";
import { cn } from "@/lib/cn";
import type { Category, Feed, Item } from "@/lib/types";
import { Button } from "./ui";

export function ItemCard({
    item,
    category,
    feed,
    onComplete,
    onDrop,
    onReopen,
    onEdit,
    onDragStart,
    onDragEnd
}: {
    item: Item;
    category?: Category;
    feed?: Feed;
    onComplete: (rect: DOMRect) => void;
    onDrop: () => void;
    onReopen: () => void;
    onEdit: () => void;
    onDragStart: () => void;
    onDragEnd: () => void;
}) {
    const done = item.status === "done";
    const dropped = item.status === "dropped";
    const isEvent = item.kind === "event";
    const course = item.class ? feed?.classes?.find((c) => c.code === item.class) : undefined;
    const color = course?.color ?? category?.color ?? feed?.color ?? "var(--color-muted)";

    return (
        <article
            draggable
            onClick={onEdit}
            onKeyDown={(e) => {
                if (e.key === "Enter") onEdit();
            }}
            tabIndex={0}
            role="button"
            onDragStart={(e) => {
                e.dataTransfer.effectAllowed = "move";
                e.dataTransfer.setData("text/plain", item.id);
                onDragStart();
            }}
            onDragEnd={onDragEnd}
            className={cn(
                "group/item animate-rise relative cursor-grab overflow-hidden rounded-xl border px-2.5 py-2 text-left transition-all duration-200 active:cursor-grabbing",
                done && "border-green/20 bg-green/5",
                dropped && "border-dashed border-line bg-base/40",
                !done && !dropped && "border-line bg-raise/70 hover:border-edge hover:bg-raise"
            )}
        >
            <span
                aria-hidden
                className={cn(
                    "absolute inset-y-1.5 left-0 w-[3px] rounded-full",
                    (done || dropped) && "opacity-40"
                )}
                style={{ background: done ? "var(--color-green)" : color }}
            />

            <div className="pl-2">
                <h4
                    className={cn(
                        "pr-1 text-[0.82rem] leading-snug font-medium hyphens-auto",
                        done && "text-dim line-through decoration-green/50",
                        dropped && "text-muted line-through",
                        !done && !dropped && "text-bright"
                    )}
                >
                    {item.title}
                </h4>

                {item.notes && (
                    <p className="mt-1 line-clamp-2 text-[0.72rem] leading-relaxed text-dim">
                        {item.notes}
                    </p>
                )}

                <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[0.66rem] empty:mt-0">
                    {(item.start || isEvent) && (
                        <span className="inline-flex items-center gap-1 font-mono text-dim">
                            {isEvent ? (
                                <CalendarClock className="size-3" />
                            ) : (
                                <Clock className="size-3" />
                            )}
                            {item.start
                                ? `${item.start}${item.end ? `–${item.end}` : ""}`
                                : "event"}
                        </span>
                    )}
                    {category && (
                        <span
                            className="inline-flex items-center rounded-full px-1.5 py-0.5 font-medium"
                            style={{
                                color: category.color,
                                background: `color-mix(in oklab, ${category.color} 14%, transparent)`
                            }}
                        >
                            {category.name}
                        </span>
                    )}
                    {feed && (
                        <span
                            className="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 font-medium"
                            style={{
                                color: course?.color ?? feed.color,
                                background: `color-mix(in oklab, ${course?.color ?? feed.color} 14%, transparent)`
                            }}
                            title={
                                course
                                    ? `${course.code}, imported from ${feed.name}`
                                    : `imported from ${feed.name}`
                            }
                        >
                            <CalendarDays className="size-3" />
                            {course?.code ?? feed.name}
                        </span>
                    )}
                    {item.series && (
                        <span className="text-muted" title="repeats">
                            <Repeat className="size-3" />
                        </span>
                    )}
                    {!!item.carried && (
                        <span
                            className="inline-flex items-center gap-1 rounded-full bg-amber/10 px-1.5 py-0.5 font-mono font-medium text-amber"
                            title={`carried forward ${item.carried} ${item.carried === 1 ? "day" : "days"}`}
                        >
                            <RotateCcw className="size-3" />
                            {item.carried}
                        </span>
                    )}
                </div>
            </div>

            {/* the two decisions a task ever needs, floated over its corner so a
                narrow column spends all of its width on the words. an event is
                not finished or abandoned, it simply happens, so it has none. */}
            <div
                hidden={isEvent}
                className="absolute top-1 right-1 flex items-center gap-0.5 rounded-lg border border-line/80 bg-raise p-0.5 opacity-0 shadow-lg transition-opacity group-focus-within/item:opacity-100 group-hover/item:opacity-100"
                onClick={(e) => e.stopPropagation()}
            >
                {done || dropped ? (
                    <Button variant="ghost" size="icon-sm" title="put it back" onClick={onReopen}>
                        <Undo2 className="size-3.5" />
                    </Button>
                ) : (
                    <>
                        <Button
                            variant="ghost"
                            size="icon-sm"
                            title="done"
                            className="text-green/70 hover:bg-green/10 hover:text-green"
                            onClick={(e) => onComplete(e.currentTarget.getBoundingClientRect())}
                        >
                            <Check className="size-4" />
                        </Button>
                        <Button
                            variant="ghost"
                            size="icon-sm"
                            title="did not happen, and cannot any more"
                            className="hover:bg-overlay hover:text-soft"
                            onClick={onDrop}
                        >
                            <Ban className="size-3.5" />
                        </Button>
                    </>
                )}
            </div>
        </article>
    );
}
