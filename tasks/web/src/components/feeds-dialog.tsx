import {
    AlertTriangle,
    CalendarDays,
    Eye,
    EyeOff,
    GraduationCap,
    ListChecks,
    Plus,
    RefreshCw,
    Trash2
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/cn";
import type { Class, Feed, FeedKind } from "@/lib/types";
import { Button, Field, Modal, fieldClass } from "./ui";

const swatches = ["#89b4fa", "#94e2d5", "#a6e3a1", "#f9e2af", "#fab387", "#f38ba8", "#cba6f7"];

const kinds: { value: FeedKind; label: string; icon: typeof CalendarDays; hint: string }[] = [
    {
        value: "events",
        label: "events",
        icon: CalendarDays,
        hint: "drawn beside the day as context, never completed"
    },
    {
        value: "tasks",
        label: "tasks",
        icon: ListChecks,
        hint: "every entry becomes a real item: completable, and carried forward if you miss it"
    },
    {
        value: "canvas",
        label: "canvas",
        icon: GraduationCap,
        hint: "always work, split by course: each one gets its own colour and can be switched off"
    }
];

function ago(iso?: string) {
    if (!iso) return "never";
    const minutes = Math.round((Date.now() - new Date(iso).getTime()) / 60000);
    if (minutes < 1) return "just now";
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    return `${Math.round(hours / 24)}d ago`;
}

export function FeedsDialog({
    open,
    onClose,
    feeds,
    onAdd,
    onEdit,
    onSync,
    onRemove
}: {
    open: boolean;
    onClose: () => void;
    feeds: Feed[];
    onAdd: (body: Pick<Feed, "name" | "url" | "color" | "kind">) => Promise<void>;
    onEdit: (
        id: string,
        body: Pick<Feed, "name" | "color" | "kind"> & { classes?: Class[] }
    ) => Promise<void>;
    onSync: (id: string) => Promise<void>;
    onRemove: (id: string) => Promise<void>;
}) {
    const [name, setName] = useState("");
    const [url, setUrl] = useState("");
    const [color, setColor] = useState(swatches[0]!);
    const [kind, setKind] = useState<FeedKind>("events");
    const [busy, setBusy] = useState(false);
    const [syncing, setSyncing] = useState("");
    const [error, setError] = useState("");

    const add = async (e: React.FormEvent) => {
        e.preventDefault();
        setBusy(true);
        setError("");
        try {
            await onAdd({ name: name.trim(), url: url.trim(), color, kind });
            setName("");
            setUrl("");
        } catch (err) {
            setError(err instanceof Error ? err.message : "that calendar would not load");
        } finally {
            setBusy(false);
        }
    };

    const sync = async (id: string) => {
        setSyncing(id);
        try {
            await onSync(id);
        } finally {
            setSyncing("");
        }
    };

    return (
        <Modal
            open={open}
            onClose={onClose}
            wide
            title="calendars"
            description="subscribed ics feeds, refetched in the background and never edited here"
        >
            <div className="flex flex-col gap-4">
                <ul className="flex flex-col gap-1.5">
                    {feeds.map((feed) => (
                        <li
                            key={feed.id}
                            className="flex flex-wrap items-center gap-3 rounded-xl border border-line/70 bg-raise/40 px-3 py-2"
                        >
                            <span
                                className="size-2.5 shrink-0 rounded-full"
                                style={{ background: feed.color }}
                            />
                            <div className="min-w-0 flex-1">
                                <p className="truncate text-sm text-bright">{feed.name}</p>
                                <p className="truncate font-mono text-[0.65rem] text-muted">
                                    {feed.url}
                                </p>
                                {feed.error ? (
                                    <p className="mt-0.5 flex items-center gap-1 text-[0.68rem] text-red">
                                        <AlertTriangle className="size-3" />
                                        {feed.error}
                                    </p>
                                ) : (
                                    <p className="mt-0.5 text-[0.68rem] text-dim">
                                        {feed.events} entries · synced {ago(feed.synced)}
                                    </p>
                                )}
                            </div>

                            <KindPicker
                                kind={feed.kind}
                                onPick={(next) =>
                                    void onEdit(feed.id, {
                                        name: feed.name,
                                        color: feed.color,
                                        kind: next
                                    })
                                }
                            />

                            <Button
                                variant="ghost"
                                size="icon-sm"
                                title="sync now"
                                onClick={() => void sync(feed.id)}
                            >
                                <RefreshCw
                                    className={cn(
                                        "size-3.5",
                                        syncing === feed.id && "animate-spin"
                                    )}
                                />
                            </Button>
                            <Button
                                variant="danger"
                                size="icon-sm"
                                title="unsubscribe, dropping anything of its own still open"
                                onClick={() => void onRemove(feed.id)}
                            >
                                <Trash2 className="size-3.5" />
                            </Button>

                            {feed.kind === "canvas" && (
                                <ul className="flex w-full flex-wrap gap-1.5 border-t border-line/60 pt-2">
                                    {(feed.classes ?? []).map((course) => (
                                        <li
                                            key={course.code}
                                            className={cn(
                                                "flex items-center gap-1.5 rounded-lg border border-line/70 bg-base/50 py-1 pr-1 pl-2",
                                                course.hidden && "opacity-45"
                                            )}
                                        >
                                            <input
                                                type="color"
                                                value={course.color}
                                                onChange={(e) =>
                                                    void onEdit(feed.id, {
                                                        name: feed.name,
                                                        color: feed.color,
                                                        kind: feed.kind,
                                                        classes: [
                                                            { ...course, color: e.target.value }
                                                        ]
                                                    })
                                                }
                                                className="size-4 shrink-0 cursor-pointer rounded-full border-0 bg-transparent p-0 [&::-webkit-color-swatch-wrapper]:p-0 [&::-webkit-color-swatch]:rounded-full [&::-webkit-color-swatch]:border-0"
                                                title={`recolour ${course.code}`}
                                            />
                                            <span className="font-mono text-[0.68rem] text-soft">
                                                {course.code}
                                            </span>
                                            <Button
                                                variant="ghost"
                                                size="icon-sm"
                                                title={
                                                    course.hidden
                                                        ? "show this course"
                                                        : "hide it, and drop what it has not done yet"
                                                }
                                                onClick={() =>
                                                    void onEdit(feed.id, {
                                                        name: feed.name,
                                                        color: feed.color,
                                                        kind: feed.kind,
                                                        classes: [
                                                            { ...course, hidden: !course.hidden }
                                                        ]
                                                    })
                                                }
                                            >
                                                {course.hidden ? (
                                                    <EyeOff className="size-3" />
                                                ) : (
                                                    <Eye className="size-3" />
                                                )}
                                            </Button>
                                        </li>
                                    ))}
                                    {(feed.classes ?? []).length === 0 && (
                                        <li className="text-[0.68rem] text-muted">
                                            no courses found in this feed yet
                                        </li>
                                    )}
                                </ul>
                            )}
                        </li>
                    ))}
                    {feeds.length === 0 && (
                        <p className="py-2 text-xs text-muted">
                            nothing subscribed. google, apple and outlook all hand out a secret ics
                            address under a calendar&apos;s sharing settings.
                        </p>
                    )}
                </ul>

                <form
                    onSubmit={add}
                    className="flex flex-col gap-3 rounded-xl border border-line bg-base/40 p-3"
                >
                    <div className="grid gap-3 sm:grid-cols-[1fr_2fr]">
                        <Field label="name">
                            <input
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                placeholder="school"
                                className={fieldClass}
                            />
                        </Field>
                        <Field label="ics or webcal url">
                            <input
                                value={url}
                                onChange={(e) => setUrl(e.target.value)}
                                placeholder="https://calendar.google.com/calendar/ical/…/basic.ics"
                                className={cn(fieldClass, "font-mono text-xs")}
                            />
                        </Field>
                    </div>

                    <Field label="bring it in as">
                        <div className="flex flex-wrap items-center gap-2">
                            <KindPicker kind={kind} onPick={setKind} />
                            <span className="text-[0.7rem] text-dim">
                                {kinds.find((k) => k.value === kind)?.hint}
                            </span>
                        </div>
                    </Field>

                    <div className="flex flex-wrap items-center gap-1.5">
                        {swatches.map((swatch) => (
                            <button
                                key={swatch}
                                type="button"
                                onClick={() => setColor(swatch)}
                                style={{ background: swatch }}
                                className={cn(
                                    "size-6 cursor-pointer rounded-full transition-transform",
                                    color === swatch ? "scale-110" : "opacity-70 hover:opacity-100"
                                )}
                                aria-label={swatch}
                            />
                        ))}
                        <Button
                            type="submit"
                            variant="primary"
                            size="sm"
                            disabled={busy || !url.trim()}
                            className="ml-auto"
                        >
                            <Plus className="size-3.5" />
                            {busy ? "fetching…" : "subscribe"}
                        </Button>
                    </div>

                    {error && <p className="text-xs text-red">{error}</p>}
                </form>
            </div>
        </Modal>
    );
}

function KindPicker({ kind, onPick }: { kind: FeedKind; onPick: (kind: FeedKind) => void }) {
    return (
        <div className="flex shrink-0 rounded-lg border border-line bg-base/60 p-0.5">
            {kinds.map((option) => (
                <button
                    key={option.value}
                    type="button"
                    title={option.hint}
                    onClick={() => onPick(option.value)}
                    className={cn(
                        "flex cursor-pointer items-center gap-1.5 rounded-md px-2 py-1 text-[0.7rem] font-medium transition-colors",
                        kind === option.value
                            ? "bg-accent/20 text-accent-soft"
                            : "text-dim hover:text-bright"
                    )}
                >
                    <option.icon className="size-3" />
                    {option.label}
                </button>
            ))}
        </div>
    );
}
