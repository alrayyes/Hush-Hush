<script lang="ts">
import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
import { Button } from '#lib/components/ui/button/index.js';
import {
	clampPage,
	pageRange,
	showPager,
	summaryRange,
	totalPages,
} from '#lib/pageMath.js';

// The numbered pager for a list that knows its total (alrayyes/Hush-Hush#892).
// It renders nothing when the list fits on one page. Links, not buttons, so a
// page is a URL: it survives a reload, can be shared and opens in a new tab.
// While `loading`, every control stays where it is but can't be used. A list low
// on a long page passes `keepScroll`, so paging it doesn't jump to the top
// (SvelteKit 3's `data-sveltekit-reset="false"`, which keeps the scroll position
// and the focused link, still there after the page changes).
let {
	page,
	total,
	pageSize,
	href,
	label = 'Pagination',
	loading = false,
	keepScroll = false,
}: {
	page: number;
	total: number;
	pageSize: number;
	href: (page: number) => string;
	label?: string;
	loading?: boolean;
	keepScroll?: boolean;
} = $props();

// A page past the end (a stale link, or rows deleted since) shows the last
// page, not an impossible range like "Showing 381–137".
const current = $derived(clampPage(page, total, pageSize));
const pages = $derived(totalPages(total, pageSize));
const range = $derived(summaryRange(current, pageSize, total));
const items = $derived(pageRange(current, pages));

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
				href={href(Math.max(1, current - 1))}
				data-sveltekit-reset={keepScroll ? false : undefined}
				disabled={loading || current <= 1}
			>
				<ChevronLeftIcon aria-hidden="true" /> Previous
			</Button>

			<span class="text-sm sm:hidden">Page {current} of {pages}</span>

			<ol class="m-0 hidden list-none items-center gap-1 p-0 sm:flex">
				{#each items as item, i (i)}
					<li>
						{#if item === 'gap'}
							<span class="px-1 text-muted-foreground" aria-hidden="true">…</span>
						{:else}
							<Button
								variant={item === current ? 'default' : 'outline'}
								class="min-w-8 {dim}"
								href={href(item)}
							data-sveltekit-reset={keepScroll ? false : undefined}
								disabled={loading}
								aria-current={item === current ? 'page' : undefined}
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
				href={href(Math.min(pages, current + 1))}
				data-sveltekit-reset={keepScroll ? false : undefined}
				disabled={loading || current >= pages}
			>
				Next <ChevronRightIcon aria-hidden="true" />
			</Button>
		</div>
	</nav>
{/if}
