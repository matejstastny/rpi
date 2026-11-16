import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export const cn = (...parts: ClassValue[]) => twMerge(clsx(parts));
