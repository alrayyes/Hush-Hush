<script lang="ts">
import { AlertDialog, Dialog } from 'bits-ui';
import { goto, invalidate } from '$app/navigation';
import {
	ApiError,
	type ConsumerEntry,
	deleteConsumer,
	renameConsumer,
} from '$lib/api';
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

function apiErrorMessage(err: unknown, fallback: string): string {
	return err instanceof ApiError ? err.message : fallback;
}

let renameOpen = $state(false);
let renameCurrentName = $state('');
let renameInput = $state('');
let renameError = $state('');

function openRename(consumer: ConsumerEntry) {
	renameCurrentName = consumer.name;
	renameInput = consumer.name;
	renameError = '';
	renameOpen = true;
}

async function submitRename(event: SubmitEvent) {
	event.preventDefault();
	renameError = '';

	try {
		await renameConsumer(renameCurrentName, renameInput);
		renameOpen = false;
		await invalidate('app:consumers');
	} catch (err) {
		renameError = apiErrorMessage(err, 'Failed to rename the consumer.');
	}
}

let deleteOpen = $state(false);
let deleteName = $state('');
let deleteError = $state('');

function openDelete(consumer: ConsumerEntry) {
	deleteName = consumer.name;
	deleteError = '';
	deleteOpen = true;
}

async function confirmDelete() {
	deleteError = '';

	try {
		await deleteConsumer(deleteName);
		deleteOpen = false;
		await invalidate('app:consumers');
	} catch (err) {
		deleteError = apiErrorMessage(err, 'Failed to delete the consumer.');
	}
}
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
					<th scope="col">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each data.consumers as consumer (consumer.name)}
					<tr>
						<td data-label="Consumer">
							<a href={secretsOverviewHref(consumer.name)}>{consumer.name}</a>
						</td>
						<td data-label="Secrets">{consumer.secret_count}</td>
						<td data-label="Actions" class="row-actions">
							<button type="button" onclick={() => openRename(consumer)}>
								Rename
							</button>
							<button
								type="button"
								class="danger"
								onclick={() => openDelete(consumer)}
							>
								Delete
							</button>
						</td>
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

<Dialog.Root bind:open={renameOpen}>
	<Dialog.Portal>
		<Dialog.Overlay class="overlay" />
		<Dialog.Content class="dialog">
			<Dialog.Title>Rename consumer</Dialog.Title>
			<form onsubmit={submitRename}>
				<label for="rename-consumer-name">Name</label>
				<input id="rename-consumer-name" bind:value={renameInput} required />

				{#if renameError}
					<p role="alert" class="error">{renameError}</p>
				{/if}

				<div class="actions">
					<Dialog.Close type="button">Cancel</Dialog.Close>
					<button type="submit">Save</button>
				</div>
			</form>
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>

<AlertDialog.Root bind:open={deleteOpen}>
	<AlertDialog.Portal>
		<AlertDialog.Overlay class="overlay" />
		<AlertDialog.Content class="dialog">
			<AlertDialog.Title>Delete "{deleteName}"?</AlertDialog.Title>
			<AlertDialog.Description>
				This removes the consumer from the directory. Secrets that still list
				it as a user aren't deleted.
			</AlertDialog.Description>
			{#if deleteError}
				<p role="alert" class="error">{deleteError}</p>
			{/if}
			<div class="actions">
				<AlertDialog.Cancel type="button">Cancel</AlertDialog.Cancel>
				<AlertDialog.Action type="button" onclick={confirmDelete}>
					Delete
				</AlertDialog.Action>
			</div>
		</AlertDialog.Content>
	</AlertDialog.Portal>
</AlertDialog.Root>

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

	.row-actions button {
		margin-right: var(--space-2);
	}

	.error {
		color: var(--color-error);
	}

	form label {
		display: block;
		margin-top: var(--space-3);
	}

	form input {
		width: 100%;
		box-sizing: border-box;
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
		margin-top: var(--space-4);
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
