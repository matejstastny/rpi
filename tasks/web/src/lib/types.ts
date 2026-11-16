export type Status = "open" | "done" | "dropped";

export type Freq = "weekly" | "monthly" | "yearly";

/** a calendar is context beside the day, the work itself, or canvas, which is
 *  always work and always several courses at once */
export type FeedKind = "events" | "tasks" | "canvas";

/** one course inside a canvas feed */
export type Class = {
    code: string;
    color: string;
    hidden: boolean;
};

/** a task waits for you and carries forward; an event happens or it doesn't */
export type Kind = "task" | "event";

export type Category = {
    id: string;
    name: string;
    color: string;
};

export type Item = {
    id: string;
    title: string;
    notes?: string;
    date: string;
    start?: string;
    end?: string;
    category?: string;
    kind: Kind;
    status: Status;
    resolved?: string;
    carried?: number;
    series?: string;
    feed?: string;
    source?: string;
    class?: string;
    occurs?: string;
    made: string;
};

export type Series = {
    id: string;
    title: string;
    notes?: string;
    start?: string;
    end?: string;
    category?: string;
    kind: Kind;
    freq: Freq;
    weekdays?: number[];
    from: string;
    until: string;
    made: string;
};

export type Feed = {
    id: string;
    name: string;
    url: string;
    color: string;
    kind: FeedKind;
    classes?: Class[];
    made: string;
    synced?: string;
    error?: string;
    events: number;
};

export type CalendarEvent = {
    feed: string;
    name: string;
    color: string;
    title: string;
    where?: string;
    date: string;
    start?: string;
    end?: string;
};

export type Day = {
    date: string;
    items: Item[];
    events: CalendarEvent[];
    today: boolean;
    past: boolean;
    planned: boolean;
};

export type Week = {
    start: string;
    end: string;
    today: string;
    days: Day[];
    stats: {
        done: number;
        dropped: number;
        open: number;
        total: number;
        streak: number;
        carried: number;
    };
    categories: Category[];
    series: Series[];
    feeds: Feed[];
};

/** what the new/edit form sends, for a single item or a whole repeat */
export type ItemDraft = {
    title: string;
    notes: string;
    date: string;
    start: string;
    end: string;
    category: string;
    kind: Kind;
    repeat: { freq: Freq; weekdays: number[]; until: string } | null;
};
