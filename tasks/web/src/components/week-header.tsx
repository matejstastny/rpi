import {
    CalendarDays,
    ChevronLeft,
    ChevronRight,
    Flame,
    Plus,
    RotateCcw,
    Sparkles,
    Tags
} from "lucide-react";
import { cn } from "@/lib/cn";
import { spanLabel, weekOffset } from "@/lib/dates";
import type { Week } from "@/lib/types";
import { Button, Ring } from "./ui";

function when(offset: number) {
    if (offset === 0) return "this week";
    if (offset === 1) return "next week";
    if (offset === -1) return "last week";
    return offset > 0 ? `${offset} weeks ahead` : `${Math.abs(offset)} weeks back`;
}

// every control on this row is the same height, so the right side reads as one
// instrument panel rather than four unrelated widgets
const pill = "flex h-9 items-center gap-2 rounded-xl border border-line bg-surface/60 px-3";

export function WeekHeader({
    week,
    onPrev,
    onNext,
    onToday,
    onNew,
    onCategories,
    onFeeds
}: {
    week: Week;
    onPrev: () => void;
    onNext: () => void;
    onToday: () => void;
    onNew: () => void;
    onCategories: () => void;
    onFeeds: () => void;
}) {
    const offset = weekOffset(week.start, week.today);
    const { done, total, open, streak, carried } = week.stats;

    return (
        <header className="flex flex-wrap items-center justify-between gap-x-6 gap-y-4">
            <div>
                <h1 className="text-xl font-semibold tracking-tight text-bright">
                    {spanLabel(week.start, week.end)}
                </h1>
                <p className={cn("mt-0.5 text-xs", offset === 0 ? "text-accent-soft" : "text-dim")}>
                    {when(offset)}
                </p>
            </div>

            <div className="flex flex-wrap items-center gap-2">
                <div className={pill}>
                    <Ring done={done} total={total} />
                    <span className="font-mono text-xs text-bright">
                        {done}
                        <span className="text-muted">/{total}</span>
                    </span>
                    <span className="text-[0.7rem] text-dim">
                        {total === 0 ? "nothing yet" : open === 0 ? "all settled" : `${open} left`}
                    </span>
                </div>

                <div
                    className={cn(pill, streak > 0 && "border-amber/25 bg-amber/5 text-amber")}
                    title="days in a row with something finished"
                >
                    <Flame className={cn("size-4", streak === 0 && "text-muted")} />
                    <span className="font-mono text-xs">{streak}</span>
                </div>

                {carried > 0 && (
                    <div
                        className={cn(pill, "border-amber/20 bg-amber/5 text-amber")}
                        title="items that followed you into today"
                    >
                        <RotateCcw className="size-4" />
                        <span className="font-mono text-xs">{carried}</span>
                    </div>
                )}

                <span aria-hidden className="mx-1 h-5 w-px bg-line max-sm:hidden" />

                <div className="flex h-9 items-center overflow-hidden rounded-xl border border-line bg-surface/60">
                    <button
                        onClick={onPrev}
                        title="previous week"
                        className="grid h-full w-9 cursor-pointer place-items-center text-dim transition-colors hover:bg-overlay/70 hover:text-bright"
                    >
                        <ChevronLeft className="size-4" />
                    </button>
                    <button
                        onClick={onToday}
                        disabled={offset === 0}
                        className="h-full cursor-pointer border-x border-line px-3 text-xs font-medium text-dim transition-colors hover:bg-overlay/70 hover:text-bright disabled:cursor-default disabled:text-muted disabled:hover:bg-transparent"
                    >
                        today
                    </button>
                    <button
                        onClick={onNext}
                        title="next week"
                        className="grid h-full w-9 cursor-pointer place-items-center text-dim transition-colors hover:bg-overlay/70 hover:text-bright"
                    >
                        <ChevronRight className="size-4" />
                    </button>
                </div>

                <Button variant="outline" size="icon" onClick={onCategories} title="categories">
                    <Tags className="size-4" />
                </Button>
                <Button variant="outline" size="icon" onClick={onFeeds} title="calendars">
                    <CalendarDays className="size-4" />
                </Button>
                <Button variant="primary" onClick={onNew} className="gap-1.5">
                    <Plus className="size-4" />
                    new
                </Button>
            </div>
        </header>
    );
}

/** the sunday prompt: the whole point of the app is the hour you spend laying
 *  the next week out, so on sunday it asks for it */
export function PlanningNudge({ onPlan }: { onPlan: () => void }) {
    return (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-accent/25 bg-gradient-to-r from-accent/12 to-transparent px-4 py-3">
            <p className="flex items-center gap-2 text-sm text-soft">
                <Sparkles className="size-4 text-accent-soft" />
                it is sunday. lay next week out while it is still quiet.
            </p>
            <Button variant="primary" size="sm" onClick={onPlan} className="gap-1.5">
                plan next week
                <ChevronRight className="size-3.5" />
            </Button>
        </div>
    );
}
