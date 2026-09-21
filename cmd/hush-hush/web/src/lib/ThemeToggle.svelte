<script lang="ts">
import { onMount } from 'svelte';
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
</script>

<button type="button" onclick={toggle} aria-pressed={theme === 'dark'}>
	{theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
</button>

<style>
	button {
		margin-left: auto;
	}
</style>
