<script lang="ts">
import { Input } from '$lib/components/ui/input/index.js';
import {
	CONSUMER_LIST_FILTER_MIN,
	consumersHref,
	filterConsumers,
} from '$lib/consumers';

// Every consumer of one variant, as links to the consumers directory. A
// list past CONSUMER_LIST_FILTER_MIN gets a filter box and a count, and the
// list scrolls in its own box, so a hundred consumers never lengthen the
// page or the dialog around it.
let { consumers, id }: { consumers: string[]; id: string } = $props();

let query = $state('');
const filterable = $derived(consumers.length > CONSUMER_LIST_FILTER_MIN);
const visible = $derived(filterConsumers(consumers, query));
</script>

<div class="space-y-2">
	{#if filterable}
		<Input
			id="{id}-filter"
			type="search"
			class="w-full"
			placeholder="Filter consumers"
			aria-label="Filter consumers"
			bind:value={query}
		/>
		<p class="m-0 text-sm text-text-muted" role="status">
			{visible.length} of {consumers.length} shown
		</p>
	{/if}
	{#if visible.length === 0}
		<p class="m-0">No consumers match &ldquo;{query}&rdquo;.</p>
	{:else}
		<ul class="m-0 max-h-64 list-none space-y-1 overflow-y-auto p-0">
			{#each visible as consumer (consumer)}
				<li class="break-all font-mono text-sm">
					<a href={consumersHref(1, consumer)}>{consumer}</a>
				</li>
			{/each}
		</ul>
	{/if}
</div>
