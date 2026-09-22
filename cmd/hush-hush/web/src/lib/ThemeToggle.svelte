<script lang="ts">
import { onMount } from 'svelte';
import { Button } from '$lib/components/ui/button/index.js';
import {
	applyTheme,
	readStoredTheme,
	resolveTheme,
	storeTheme,
	type Theme,
} from '$lib/theme';

let theme = $state<Theme>('light');

onMount(() => {
	const attr = document.documentElement.getAttribute('data-theme');

	theme =
		attr === 'dark' || attr === 'light'
			? attr
			: resolveTheme(
					readStoredTheme(),
					window.matchMedia('(prefers-color-scheme: dark)').matches,
				);
});

function toggle() {
	const next: Theme = theme === 'dark' ? 'light' : 'dark';

	applyTheme(next);
	storeTheme(next);
	theme = next;
}

// The label names the action a click performs, not the current state -
// "Switch to dark mode" while light, "Switch to light mode" while dark -
// so a screen reader announces what pressing it does, matching this
// button's own icon swap (alrayyes/hush-hush#301).
const label = $derived(
	theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode',
);
</script>

<Button
	variant="ghost"
	size="icon"
	class="ml-auto"
	onclick={toggle}
	aria-pressed={theme === 'dark'}
	aria-label={label}
	title={label}
>
	{#if theme === 'dark'}
		<svg
			class="h-5 w-5"
			viewBox="0 0 24 24"
			aria-hidden="true"
			focusable="false"
		>
			<path
				fill="currentColor"
				d="M12 3a1 1 0 0 1 1 1v1a1 1 0 1 1-2 0V4a1 1 0 0 1 1-1Zm0 4a5 5 0 1 1 0 10 5 5 0 0 1 0-10Zm0 2a3 3 0 1 0 0 6 3 3 0 0 0 0-6Zm9 3a1 1 0 0 1-1 1h-1a1 1 0 1 1 0-2h1a1 1 0 0 1 1 1ZM5 12a1 1 0 0 1-1 1H3a1 1 0 1 1 0-2h1a1 1 0 0 1 1 1Zm14.071-6.071a1 1 0 0 1 0 1.414l-.707.707a1 1 0 1 1-1.415-1.414l.708-.708a1 1 0 0 1 1.414 0ZM7.05 17.657a1 1 0 0 1 0 1.414l-.707.707a1 1 0 1 1-1.415-1.414l.708-.708a1 1 0 0 1 1.414 0Zm11.314 1.414a1 1 0 0 1-1.414 0l-.708-.707a1 1 0 0 1 1.415-1.415l.707.708a1 1 0 0 1 0 1.414ZM6.343 6.343a1 1 0 0 1-1.414 0l-.708-.707a1 1 0 0 1 1.415-1.415l.707.708a1 1 0 0 1 0 1.414ZM12 19a1 1 0 0 1 1 1v1a1 1 0 1 1-2 0v-1a1 1 0 0 1 1-1Z"
			/>
		</svg>
	{:else}
		<svg
			class="h-5 w-5"
			viewBox="0 0 24 24"
			aria-hidden="true"
			focusable="false"
		>
			<path
				fill="currentColor"
				d="M20.354 15.354A9 9 0 0 1 8.646 3.646a9.003 9.003 0 1 0 11.708 11.708Z"
			/>
		</svg>
	{/if}
</Button>
