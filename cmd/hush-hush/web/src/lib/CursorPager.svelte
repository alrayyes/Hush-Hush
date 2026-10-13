<script lang="ts">
import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
import { Button } from '#lib/components/ui/button/index.js';
import { showCursorSizeSelector } from '#lib/pageSize.js';
import RowsPerPage from '#lib/RowsPerPage.svelte';

// The pager for a list read by cursor, the audit log (alrayyes/Hush-Hush#892).
// It has no total and no page numbers: the server only says whether there is a
// newer or an older page. It renders nothing when there is neither. These are
// buttons because a cursor is the page's own bookkeeping, not a URL.
let {
	count,
	canNewer,
	canOlder,
	onNewer,
	onOlder,
	label = 'Pagination',
	loading = false,
	size,
	sizeOptions,
	onSizeChange,
}: {
	count: number;
	canNewer: boolean;
	canOlder: boolean;
	onNewer: () => void;
	onOlder: () => void;
	label?: string;
	loading?: boolean;
	size?: number;
	sizeOptions?: readonly number[];
	onSizeChange?: (size: number) => void;
} = $props();

const controls = $derived(canNewer || canOlder);
const bar = $derived(
	controls ||
		(sizeOptions !== undefined &&
			size !== undefined &&
			showCursorSizeSelector(size, sizeOptions, canNewer, canOlder)),
);
</script>

{#if bar}
	<nav
		aria-label={label}
		aria-busy={loading}
		class="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between"
	>
		<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
			<p class="text-center text-sm text-muted-foreground sm:text-left">
				Showing <span class="font-medium text-foreground">{count}</span>
				{count === 1 ? 'entry' : 'entries'}
			</p>

			{#if sizeOptions && size !== undefined && onSizeChange}
				<RowsPerPage options={sizeOptions} {size} onchange={onSizeChange} disabled={loading} />
			{/if}
		</div>

		{#if controls}
		<div class="flex items-center justify-between gap-2 sm:justify-end">
			<Button
				variant="outline"
				class="h-11 min-w-11 sm:h-8 sm:min-w-0"
				onclick={onNewer}
				disabled={loading || !canNewer}
			>
				<ArrowUpIcon aria-hidden="true" /> Newer
			</Button>
			<Button
				variant="outline"
				class="h-11 min-w-11 sm:h-8 sm:min-w-0"
				onclick={onOlder}
				disabled={loading || !canOlder}
			>
				Older <ArrowDownIcon aria-hidden="true" />
			</Button>
		</div>
		{/if}
	</nav>
{/if}
