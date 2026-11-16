import { X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { cn } from "@/lib/cn";
import { parseClock } from "@/lib/dates";

type ButtonProps = React.ComponentProps<"button"> & {
    variant?: "primary" | "outline" | "ghost" | "soft" | "danger";
    size?: "sm" | "md" | "icon" | "icon-sm";
};

const variants: Record<NonNullable<ButtonProps["variant"]>, string> = {
    primary:
        "bg-accent text-bright shadow-[0_6px_20px_-10px] shadow-accent/90 hover:bg-accent-alt active:translate-y-px",
    outline:
        "border border-line bg-raise/60 hover:border-edge hover:bg-overlay text-soft hover:text-bright",
    ghost: "text-dim hover:bg-overlay/70 hover:text-bright",
    soft: "bg-overlay/70 text-soft hover:bg-overlay hover:text-bright",
    danger: "text-red/80 hover:bg-red/10 hover:text-red"
};

const sizes: Record<NonNullable<ButtonProps["size"]>, string> = {
    sm: "h-7 gap-1.5 rounded-lg px-2.5 text-xs",
    md: "h-9 gap-2 rounded-xl px-3.5 text-sm",
    icon: "size-9 rounded-xl",
    "icon-sm": "size-7 rounded-lg"
};

export function Button({ className, variant = "soft", size = "md", ...props }: ButtonProps) {
    return (
        <button
            className={cn(
                "inline-flex shrink-0 cursor-pointer items-center justify-center font-medium whitespace-nowrap transition-all duration-150 select-none",
                "disabled:pointer-events-none disabled:opacity-40 [&_svg]:shrink-0",
                variants[variant],
                sizes[size],
                className
            )}
            {...props}
        />
    );
}

export const fieldClass =
    "w-full rounded-xl border border-line bg-base/60 px-3 py-2 text-sm text-bright placeholder:text-muted transition-colors hover:border-edge focus:border-accent/70 focus:outline-none";

export function Field({
    label,
    hint,
    children
}: {
    label: string;
    hint?: string;
    children: ReactNode;
}) {
    return (
        <label className="flex flex-col gap-1.5">
            <span className="flex items-baseline justify-between text-xs font-medium tracking-wide text-dim uppercase">
                {label}
                {hint && <span className="text-[0.65rem] normal-case text-muted">{hint}</span>}
            </span>
            {children}
        </label>
    );
}

/** a time field that takes shorthand rather than a locale's idea of a clock.
 *  the native time input was the one place the app showed am/pm, and it made
 *  you tab between an hour box and a minute box to say 18:00. this is one box
 *  that reads 1330, 430, 4.30 or 13 and settles on 24 hour time when it loses
 *  focus. */
export function TimeField({
    value,
    onChange,
    onValidity,
    disabled,
    placeholder = "--:--"
}: {
    value: string;
    onChange: (next: string) => void;
    /** so the form around it can refuse to save what the box is showing */
    onValidity?: (ok: boolean) => void;
    disabled?: boolean;
    placeholder?: string;
}) {
    const [text, setText] = useState(value);
    const [bad, setBad] = useState(false);

    useEffect(() => {
        setText(value);
        setBad(false);
    }, [value]);

    const settle = () => {
        const parsed = parseClock(text);
        if (parsed === null) {
            setBad(true);
            onValidity?.(false);
            return;
        }
        setBad(false);
        onValidity?.(true);
        setText(parsed);
        if (parsed !== value) onChange(parsed);
    };

    return (
        <input
            value={text}
            disabled={disabled}
            inputMode="numeric"
            placeholder={placeholder}
            aria-invalid={bad}
            onChange={(e) => {
                setText(e.target.value);
                if (bad) {
                    setBad(false);
                    onValidity?.(true);
                }
            }}
            onBlur={settle}
            onKeyDown={(e) => {
                // enter submits the form around this field, so settle first
                if (e.key === "Enter") settle();
            }}
            className={cn(
                fieldClass,
                "font-mono tabular-nums",
                disabled && "opacity-40",
                bad && "border-red/70 text-red"
            )}
        />
    );
}

export function Modal({
    open,
    onClose,
    title,
    description,
    children,
    footer,
    wide
}: {
    open: boolean;
    onClose: () => void;
    title: string;
    description?: string;
    children: ReactNode;
    footer?: ReactNode;
    wide?: boolean;
}) {
    useEffect(() => {
        if (!open) return;
        const escape = (e: KeyboardEvent) => e.key === "Escape" && onClose();
        document.addEventListener("keydown", escape);
        document.body.style.overflow = "hidden";
        return () => {
            document.removeEventListener("keydown", escape);
            document.body.style.overflow = "";
        };
    }, [open, onClose]);

    if (!open) return null;

    return (
        <div className="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-6">
            <button
                aria-label="close"
                className="absolute inset-0 cursor-default bg-base/70 backdrop-blur-sm"
                onClick={onClose}
            />
            <div
                role="dialog"
                aria-modal
                aria-label={title}
                className={cn(
                    "animate-sweep relative flex max-h-[90dvh] w-full flex-col overflow-hidden rounded-t-3xl border border-line bg-surface shadow-[0_30px_80px_-20px_rgb(0_0_0/0.7)] sm:rounded-3xl",
                    wide ? "sm:max-w-2xl" : "sm:max-w-lg"
                )}
            >
                <header className="flex items-start justify-between gap-4 border-b border-line/70 px-5 py-4">
                    <div>
                        <h2 className="text-base font-semibold text-bright">{title}</h2>
                        {description && <p className="mt-0.5 text-xs text-dim">{description}</p>}
                    </div>
                    <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="close">
                        <X className="size-4" />
                    </Button>
                </header>

                <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>

                {footer && (
                    <footer className="flex items-center justify-end gap-2 border-t border-line/70 bg-raise/40 px-5 py-3.5">
                        {footer}
                    </footer>
                )}
            </div>
        </div>
    );
}

/** the week's completion, drawn as a ring so progress is felt rather than read */
export function Ring({ done, total }: { done: number; total: number }) {
    const ratio = total === 0 ? 0 : done / total;
    const radius = 16;
    const circumference = 2 * Math.PI * radius;

    return (
        <svg viewBox="0 0 40 40" className="size-5 -rotate-90" aria-hidden>
            <circle
                cx="20"
                cy="20"
                r={radius}
                fill="none"
                stroke="var(--color-line)"
                strokeWidth="6"
            />
            <circle
                cx="20"
                cy="20"
                r={radius}
                fill="none"
                stroke="url(#ring)"
                strokeWidth="6"
                strokeLinecap="round"
                strokeDasharray={circumference}
                strokeDashoffset={circumference * (1 - ratio)}
                className="transition-[stroke-dashoffset] duration-500 ease-out"
            />
            <defs>
                <linearGradient id="ring" x1="0" y1="0" x2="1" y2="1">
                    <stop offset="0%" stopColor="var(--color-accent)" />
                    <stop offset="100%" stopColor="var(--color-accent-soft)" />
                </linearGradient>
            </defs>
        </svg>
    );
}
