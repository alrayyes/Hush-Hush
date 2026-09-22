<script lang="ts">
import { Card, CardContent } from '$lib/components/ui/card/index.js';
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
		<Card>
			<CardContent>
				<div class="changelog-content break-words">
					{@html renderChangelog(data.changelog)}
				</div>
			</CardContent>
		</Card>
	{:else}
		<p>No changelog available.</p>
	{/if}
</main>
