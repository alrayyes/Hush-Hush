<script lang="ts">
import GithubIcon from '$lib/GithubIcon.svelte';

let { version }: { version: string } = $props();

// Every footer link is a 44px target, so a phone can hit any of them.
const link = 'inline-flex min-h-11 items-center text-inherit';
</script>

<!-- Spaced, not dot-separated: a "·" can't be kept off the end of a row once
     the items wrap on a phone. -->
<footer
	class="mx-auto mt-12 mb-6 flex max-w-content flex-wrap items-center justify-center gap-x-4 px-4 text-sm text-text-muted"
>
	<a href="/changelog" class={link}>{version}</a>
	<a href="/disclaimer" class={link}>Disclaimer</a>
	<a href="/privacy" class={link}>Privacy</a>
	<a href="/license" class={link}>GPL-3.0</a>
	<a
		href="https://github.com/alrayyes/Hush-Hush"
		target="_blank"
		rel="noopener noreferrer"
		class="{link} gap-1"
	>
		<GithubIcon class="size-3.5" />
		GitHub
		<span class="sr-only">(external site)</span>
	</a>
</footer>
