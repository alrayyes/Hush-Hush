<script lang="ts">
import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
import { Button } from '#lib/components/ui/button/index.js';
import { pageRange, showPager, summaryRange, totalPages } from '#lib/pageMath.js';

// The numbered pager for a list that knows its total (alrayyes/Hush-Hush#892).
// It renders nothing when the list fits on one page. Links, not buttons, so a
// page is a URL: it survives a reload, can be shared and opens in a new tab.
// While `loading`, every control stays where it is but can't be used.
let {
	page,
	total,
	pageSize,
	href,
	label = 'Pagination',
	loading = false,
}: {
	page: number;
	total: number;
	pageSize: number;
	href: (page: number) => string;
	label?: string;
	loading?: boolean;
} = $props();

const pages = $derived(totalPages(total, pageSize));
const range = $derived(summaryRange(page, pageSize, total));
const items = $derived(pageRange(page, pages));

// A disabled link has no :disabled state, so the button's own dimming never
// reaches it. aria-disabled is what Button sets on a link, so style that.
const dim = 'aria-disabled:pointer-events-none aria-disabled:opacity-50';
</script>

{#if showPager(total, pageSize)}
	<nav
		aria-label={label}
		aria-busy={loading}
		class="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between"
	>
		<p class="text-center text-sm text-muted-foreground sm:text-left">
			Showing <span class="font-medium text-foreground">{range.from}–{range.to}</span>
			of <span class="font-medium text-foreground">{total}</span>
		</p>

		<div class="flex items-center justify-between gap-2 sm:justify-end">
			<Button
				variant="outline"
				class="h-11 min-w-11 sm:h-8 sm:min-w-0 {dim}"
				href={href(Math.max(1, page - 1))}
				disabled={loading || page <= 1}
			>
				<ChevronLeftIcon aria-hidden="true" /> Previous
			</Button>

			<span class="text-sm sm:hidden">Page {page} of {pages}</span>

			<ol class="m-0 hidden list-none items-center gap-1 p-0 sm:flex">
				{#each items as item, i (i)}
					<li>
						{#if item === 'gap'}
							<span class="px-1 text-muted-foreground" aria-hidden="true">…</span>
						{:else}
							<Button
								variant={item === page ? 'default' : 'outline'}
								class="min-w-8 {dim}"
								href={href(item)}
								disabled={loading}
								aria-current={item === page ? 'page' : undefined}
								aria-label="Page {item}"
							>
								{item}
							</Button>
						{/if}
					</li>
				{/each}
			</ol>

			<Button
				variant="outline"
				class="h-11 min-w-11 sm:h-8 sm:min-w-0 {dim}"
				href={href(Math.min(pages, page + 1))}
				disabled={loading || page >= pages}
			>
				Next <ChevronRightIcon aria-hidden="true" />
			</Button>
		</div>
	</nav>
{/if}
