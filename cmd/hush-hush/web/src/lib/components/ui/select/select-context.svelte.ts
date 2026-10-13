import { getContext, setContext } from 'svelte';

// What a Select's trigger and its open list need to know about each other:
// the list's id, so the trigger can say it controls it, and the trigger's
// label, so the list is named by it (alrayyes/Hush-Hush#917). Root owns it.
export type SelectContext = { listId: string; labelledby: string | undefined };

const KEY = Symbol('hush-select');

export function provideSelectContext(listId: string): SelectContext {
	const context: SelectContext = $state({ listId, labelledby: undefined });

	return setContext(KEY, context);
}

export function useSelectContext(): SelectContext | undefined {
	return getContext<SelectContext | undefined>(KEY);
}
