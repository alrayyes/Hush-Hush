<script lang="ts">
import { renderChangelog } from '$lib/markdown';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();
</script>

<svelte:head>
	<title>Changelog - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-240 px-4">
	<h1>Changelog</h1>
	{#if data.changelog}
		<div class="changelog-content break-words">
			{@html renderChangelog(data.changelog)}
		</div>
	{:else}
		<p>No changelog available.</p>
	{/if}
</main>
