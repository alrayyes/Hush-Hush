<script lang="ts">
import ExternalLinkIcon from '@lucide/svelte/icons/external-link';

let { version }: { version: string } = $props();

// Every footer link is a 44px target, so a phone can hit any of them.
const link = 'inline-flex min-h-11 items-center text-inherit';
</script>

<footer
	class="mx-auto mt-12 mb-6 flex max-w-content flex-wrap items-center justify-center gap-x-2 px-4 text-sm text-text-muted"
>
	<a href="/changelog" class={link}>{version}</a>
	<span aria-hidden="true">&middot;</span>
	<a
		href="https://github.com/alrayyes/Hush-Hush"
		target="_blank"
		rel="noopener noreferrer"
		class="{link} gap-1"
	>
		Source
		<ExternalLinkIcon aria-hidden="true" class="size-3.5" />
		<span class="sr-only">(external site)</span>
	</a>
	<span aria-hidden="true">&middot;</span>
	<a href="/disclaimer" class={link}>Disclaimer</a>
	<span aria-hidden="true">&middot;</span>
	<a href="/privacy" class={link}>Privacy</a>
	<span aria-hidden="true">&middot;</span>
	<a href="/license" class={link}>GPL-3.0</a>
</footer>
