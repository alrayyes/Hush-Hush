<script lang="ts">
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();
</script>

<svelte:head>
	<title>Changelog - hush-hush</title>
</svelte:head>

<main>
	<h1>Changelog</h1>
	{#if data.changelog}
		<pre>{data.changelog}</pre>
	{:else}
		<p>No changelog available.</p>
	{/if}
</main>

<style>
	main {
		max-width: 60rem;
		margin: 2rem auto;
		padding: 0 1rem;
	}

	pre {
		white-space: pre-wrap;
		word-wrap: break-word;
	}
</style>
