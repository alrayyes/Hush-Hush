<script lang="ts">
import { Button } from '$lib/components/ui/button/index.js';
import { summariseConsumers } from '$lib/consumers';

// What a secret's row or card says about its consumers: the first two, then
// a button for the rest. Its height doesn't depend on how many there are.
let {
	consumers,
	name,
	onmore,
	touch = false,
}: {
	consumers: string[] | undefined;
	name: string;
	onmore: () => void;
	// Phone cards use 44px targets; the table's buttons stay small.
	touch?: boolean;
} = $props();

const summary = $derived(summariseConsumers(consumers));
</script>

{#if summary.shown.length > 0}
	<span class="flex flex-wrap items-center gap-x-1 gap-y-0.5 text-xs text-text-muted">
		{#each summary.shown as consumer, i (consumer)}
			<span class="inline-block max-w-full truncate font-mono" title={consumer}
				>{consumer}{i < summary.shown.length - 1 ? ',' : ''}</span
			>
		{/each}
		{#if summary.more > 0}
			<Button
				variant="outline"
				size="sm"
				class={touch ? 'min-h-11 min-w-11' : 'h-6 px-2 text-xs'}
				aria-label="Show all {summary.shown.length + summary.more} consumers of {name}"
				onclick={onmore}
			>
				+{summary.more} more
			</Button>
		{/if}
	</span>
{/if}
