import { Check, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/cn";
import type { Category } from "@/lib/types";
import { Button, Field, Modal, fieldClass } from "./ui";

const swatches = [
    "#cba6f7",
    "#a855c9",
    "#c47ab8",
    "#f5c2e7",
    "#89b4fa",
    "#94e2d5",
    "#a6e3a1",
    "#f9e2af",
    "#fab387",
    "#f38ba8"
];

export function CategoriesDialog({
    open,
    onClose,
    categories,
    onAdd,
    onEdit,
    onRemove
}: {
    open: boolean;
    onClose: () => void;
    categories: Category[];
    onAdd: (body: Omit<Category, "id">) => Promise<void>;
    onEdit: (id: string, body: Omit<Category, "id">) => Promise<void>;
    onRemove: (id: string) => Promise<void>;
}) {
    const [name, setName] = useState("");
    const [color, setColor] = useState(swatches[0]!);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");

    const add = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!name.trim()) return;
        setBusy(true);
        setError("");
        try {
            await onAdd({ name: name.trim(), color });
            setName("");
        } catch (err) {
            setError(err instanceof Error ? err.message : "that did not work");
        } finally {
            setBusy(false);
        }
    };

    return (
        <Modal open={open} onClose={onClose} title="categories" description="what colours the week">
            <div className="flex flex-col gap-4">
                <ul className="flex flex-col gap-1.5">
                    {categories.map((category) => (
                        <li
                            key={category.id}
                            className="group/row flex items-center gap-3 rounded-xl border border-line/70 bg-raise/40 px-3 py-2"
                        >
                            <input
                                type="color"
                                value={category.color}
                                onChange={(e) =>
                                    void onEdit(category.id, {
                                        name: category.name,
                                        color: e.target.value
                                    })
                                }
                                className="size-5 shrink-0 cursor-pointer rounded-full border-0 bg-transparent p-0 [&::-webkit-color-swatch-wrapper]:p-0 [&::-webkit-color-swatch]:rounded-full [&::-webkit-color-swatch]:border-0"
                                title="recolour"
                            />
                            <input
                                defaultValue={category.name}
                                onBlur={(e) => {
                                    const next = e.target.value.trim();
                                    if (next && next !== category.name) {
                                        void onEdit(category.id, {
                                            name: next,
                                            color: category.color
                                        });
                                    }
                                }}
                                className="min-w-0 flex-1 bg-transparent text-sm text-bright outline-none"
                            />
                            <Button
                                variant="danger"
                                size="icon-sm"
                                title="delete, anything wearing it just loses the colour"
                                onClick={() => void onRemove(category.id)}
                            >
                                <Trash2 className="size-3.5" />
                            </Button>
                        </li>
                    ))}
                    {categories.length === 0 && <p className="py-2 text-xs text-muted">none yet</p>}
                </ul>

                <form
                    onSubmit={add}
                    className="flex flex-col gap-3 rounded-xl border border-line bg-base/40 p-3"
                >
                    <Field label="new category">
                        <input
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            placeholder="school, gym, admin"
                            className={fieldClass}
                        />
                    </Field>

                    <div className="flex flex-wrap items-center gap-1.5">
                        {swatches.map((swatch) => (
                            <button
                                key={swatch}
                                type="button"
                                onClick={() => setColor(swatch)}
                                style={{ background: swatch }}
                                className={cn(
                                    "grid size-6 cursor-pointer place-items-center rounded-full transition-transform",
                                    color === swatch ? "scale-110" : "opacity-70 hover:opacity-100"
                                )}
                                aria-label={swatch}
                            >
                                {color === swatch && (
                                    <Check className="size-3.5 text-base" strokeWidth={3} />
                                )}
                            </button>
                        ))}
                        <Button
                            type="submit"
                            variant="primary"
                            size="sm"
                            disabled={busy}
                            className="ml-auto"
                        >
                            <Plus className="size-3.5" />
                            add
                        </Button>
                    </div>

                    {error && <p className="text-xs text-red">{error}</p>}
                </form>
            </div>
        </Modal>
    );
}
