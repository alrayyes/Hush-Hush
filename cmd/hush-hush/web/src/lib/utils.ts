import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export type {
	WithElementRef,
	WithoutChild,
	WithoutChildrenOrChild,
} from 'svelte-toolbelt';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}
