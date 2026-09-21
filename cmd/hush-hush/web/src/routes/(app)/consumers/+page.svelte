<script lang="ts">
import { goto } from '$app/navigation';
import {
	CONSUMERS_PAGE_SIZE,
	consumersHref,
	secretsOverviewHref,
	totalPages,
} from '$lib/consumers';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

// Seeded once from the load function's own current filter, then owned
// entirely by the input itself - a later navigation's fresh data.q is
// reflected by the browser's own back/forward restoring the field, not
// by re-syncing this state (same intentional one-time seed as
// audit-log/+page.svelte's `entries`). svelte-check's
// state_referenced_locally warning is exactly this, not a sync bug.
let filterInput = $state(data.q);

function applyFilter() {
	void goto(consumersHref(1, filterInput.trim()), {
		keepFocus: true,
		noScroll: true,
	});
}

const pages = $derived(totalPages(data.total, CONSUMERS_PAGE_SIZE));
</script>

<svelte:head>
	<title>Consumers - hush-hush</title>
</svelte:head>

<main>
	<header class="page-header">
		<h1>Consumers</h1>
	</header>

	<form
		class="filter"
		onsubmit={(event) => {
			event.preventDefault();
			applyFilter();
		}}
	>
		<label for="consumer-filter">Filter by name</label>
		<input
			id="consumer-filter"
			bind:value={filterInput}
			onchange={applyFilter}
			placeholder="homelab"
		/>
	</form>

	{#if data.consumers.length === 0}
		<p>No consumers match{data.q ? ` "${data.q}"` : ' yet'}.</p>
	{:else}
		<table class="responsive-table">
			<thead>
				<tr>
					<th scope="col">Consumer</th>
					<th scope="col">Secrets</th>
				</tr>
			</thead>
			<tbody>
				{#each data.consumers as consumer (consumer.name)}
					<tr>
						<td data-label="Consumer">
							<a href={secretsOverviewHref(consumer.name)}>{consumer.name}</a>
						</td>
						<td data-label="Secrets">{consumer.secret_count}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

	{#if pages > 1}
		<nav aria-label="Pagination" class="pagination">
			<ol>
				{#each { length: pages } as _, i (i)}
					{@const pageNumber = i + 1}
					<li>
						<a
							href={consumersHref(pageNumber, data.q)}
							aria-current={data.page === pageNumber ? 'page' : undefined}
						>
							{pageNumber}
						</a>
					</li>
				{/each}
			</ol>
		</nav>
	{/if}
</main>

<style>
	main {
		max-width: 60rem;
		margin: var(--space-8) auto;
		padding: 0 var(--space-4);
	}

	.page-header {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
		align-items: center;
		justify-content: space-between;
	}

	.filter {
		margin-bottom: 1rem;
	}

	.filter label {
		display: block;
		font-size: 0.85rem;
	}

	.filter input {
		width: 100%;
		max-width: 20rem;
		box-sizing: border-box;
	}

	.pagination ol {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
		list-style: none;
		padding: 0;
		margin-top: 1rem;
	}

	.pagination a[aria-current='page'] {
		font-weight: bold;
		text-decoration: underline;
	}
</style>
