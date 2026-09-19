<script lang="ts">
import favicon from '$lib/assets/favicon.svg';
import Footer from '$lib/Footer.svelte';
import type { LayoutData } from './$types';

let {
	data,
	children,
}: { data: LayoutData; children: import('svelte').Snippet } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}

<Footer version={data.version} />
