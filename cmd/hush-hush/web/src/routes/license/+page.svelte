<script lang="ts">
import { Card, CardContent } from '$lib/components/ui/card/index.js';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();
</script>

<svelte:head>
	<title>License - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-160 px-4">
	<h1>License</h1>
	{#if data.license}
		<Card>
			<CardContent>
				<pre class="font-sans text-base leading-relaxed whitespace-pre-wrap">{data.license}</pre>
			</CardContent>
		</Card>
	{:else}
		<p>No license text available.</p>
	{/if}
</main>
