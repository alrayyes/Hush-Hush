<script lang="ts">
import { goto, invalidate } from '$app/navigation';
import {
	ApiError,
	addConsumer,
	type ConsumerEntry,
	deleteConsumer,
	renameConsumer,
} from '$lib/api';
import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
import * as Dialog from '$lib/components/ui/dialog/index.js';
import { Input } from '$lib/components/ui/input/index.js';
import { Label } from '$lib/components/ui/label/index.js';
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

let addOpen = $state(false);
let addName = $state('');
let addError = $state('');

function resetAddForm() {
	addName = '';
	addError = '';
}

async function submitAdd(event: SubmitEvent) {
	event.preventDefault();
	addError = '';

	try {
		await addConsumer(addName);
		addOpen = false;
		resetAddForm();
		await invalidate('app:consumers');
	} catch (err) {
		addError = apiErrorMessage(err, 'Failed to add the consumer.');
	}
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

<main class="mx-auto my-8 max-w-240 px-4">
	<header class="flex flex-wrap items-center justify-between gap-2">
		<h1>Consumers</h1>
		<Dialog.Root
			bind:open={addOpen}
			onOpenChange={(open) => {
				if (!open) resetAddForm();
			}}
		>
			<Dialog.Trigger class={buttonVariants({ variant: 'default' })}>
				Add consumer
			</Dialog.Trigger>
			<Dialog.Content>
				<Dialog.Header>
					<Dialog.Title>Add a consumer</Dialog.Title>
					<Dialog.Description>
						Adds it to the directory before any secret references it.
					</Dialog.Description>
				</Dialog.Header>
				<form onsubmit={submitAdd}>
					<Label for="add-consumer-name">Name</Label>
					<Input id="add-consumer-name" class="mt-1 w-full" bind:value={addName} required />

					{#if addError}
						<p role="alert" class="mt-3 text-error">{addError}</p>
					{/if}

					<Dialog.Footer>
						<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
							Cancel
						</Dialog.Close>
						<Button type="submit">Add</Button>
					</Dialog.Footer>
				</form>
			</Dialog.Content>
		</Dialog.Root>
	</header>

	<form
		class="mb-4"
		onsubmit={(event) => {
			event.preventDefault();
			applyFilter();
		}}
	>
		<Label for="consumer-filter">Filter by name</Label>
		<Input
			id="consumer-filter"
			class="mt-1 w-full max-w-xs"
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
						<td data-label="Actions" class="row-actions gap-2">
							<Button variant="outline" size="sm" onclick={() => openRename(consumer)}>
								Rename
							</Button>
							<Button variant="destructive" size="sm" onclick={() => openDelete(consumer)}>
								Delete
							</Button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

	{#if pages > 1}
		<nav aria-label="Pagination">
			<ol class="mt-4 flex list-none flex-wrap gap-2 p-0">
				{#each { length: pages } as _, i (i)}
					{@const pageNumber = i + 1}
					<li>
						<a
							href={consumersHref(pageNumber, data.q)}
							aria-current={data.page === pageNumber ? 'page' : undefined}
							class:font-bold={data.page === pageNumber}
							class:underline={data.page === pageNumber}
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
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Rename consumer</Dialog.Title>
		</Dialog.Header>
		<form onsubmit={submitRename}>
			<Label for="rename-consumer-name">Name</Label>
			<Input id="rename-consumer-name" class="mt-1 w-full" bind:value={renameInput} required />

			{#if renameError}
				<p role="alert" class="mt-3 text-error">{renameError}</p>
			{/if}

			<Dialog.Footer>
				<Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close>
				<Button type="submit">Save</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<AlertDialog.Root bind:open={deleteOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete "{deleteName}"?</AlertDialog.Title>
			<AlertDialog.Description>
				This removes the consumer from the directory. Secrets that still list
				it as a user aren't deleted.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if deleteError}
			<p role="alert" class="text-error">{deleteError}</p>
		{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDelete}>
				Delete
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
