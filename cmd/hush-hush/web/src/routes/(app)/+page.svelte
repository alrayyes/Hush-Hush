<script lang="ts">
import { AlertDialog, Dialog } from 'bits-ui';
import { invalidate } from '$app/navigation';
import {
	ApiError,
	createObject,
	deleteObject,
	getObjectValue,
	updateObject,
} from '$lib/api';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

function parseUsedBy(raw: string): string[] | undefined {
	const items = raw
		.split(',')
		.map((s) => s.trim())
		.filter(Boolean);

	return items.length > 0 ? items : undefined;
}

function apiErrorMessage(err: unknown, fallback: string): string {
	return err instanceof ApiError ? err.message : fallback;
}

let createOpen = $state(false);
let createError = $state('');
let createId = $state('');
let createValue = $state('');
let createDescription = $state('');
let createUsedBy = $state('');

function resetCreateForm() {
	createId = '';
	createValue = '';
	createDescription = '';
	createUsedBy = '';
	createError = '';
}

async function submitCreate(event: SubmitEvent) {
	event.preventDefault();
	createError = '';

	try {
		await createObject({
			id: createId,
			value: createValue,
			description: createDescription || undefined,
			used_by: parseUsedBy(createUsedBy),
		});
		createOpen = false;
		resetCreateForm();
		await invalidate('app:objects');
	} catch (err) {
		createError = apiErrorMessage(err, 'Failed to create the secret.');
	}
}

let editOpen = $state(false);
let editId = $state('');
let editValue = $state('');
let editError = $state('');

function openEdit(id: string) {
	editId = id;
	editValue = '';
	editError = '';
	editOpen = true;
}

async function submitEdit(event: SubmitEvent) {
	event.preventDefault();
	editError = '';

	try {
		await updateObject(editId, editValue);
		editOpen = false;
		await invalidate('app:objects');
	} catch (err) {
		editError = apiErrorMessage(err, 'Failed to update the secret.');
	}
}

let viewOpen = $state(false);
let viewId = $state('');
let viewValue = $state('');
let viewError = $state('');

async function openView(id: string) {
	viewId = id;
	viewValue = '';
	viewError = '';
	viewOpen = true;

	try {
		viewValue = await getObjectValue(id);
	} catch (err) {
		viewError = apiErrorMessage(err, 'Failed to fetch the secret.');
	}
}

let deleteOpen = $state(false);
let deleteId = $state('');
let deleteError = $state('');

function openDelete(id: string) {
	deleteId = id;
	deleteError = '';
	deleteOpen = true;
}

async function confirmDelete() {
	deleteError = '';

	try {
		await deleteObject(deleteId);
		deleteOpen = false;
		await invalidate('app:objects');
	} catch (err) {
		deleteError = apiErrorMessage(err, 'Failed to delete the secret.');
	}
}
</script>

<svelte:head>
	<title>Secrets - hush-hush</title>
</svelte:head>

<main>
	<header class="page-header">
		<h1>Secrets</h1>
		<Dialog.Root bind:open={createOpen}>
			<Dialog.Trigger>New secret</Dialog.Trigger>
			<Dialog.Portal>
				<Dialog.Overlay class="overlay" />
				<Dialog.Content class="dialog">
					<Dialog.Title>Create a secret</Dialog.Title>
					<form onsubmit={submitCreate}>
						<label for="create-id">Id</label>
						<input id="create-id" bind:value={createId} required />

						<label for="create-value">Ciphertext (base64)</label>
						<textarea id="create-value" bind:value={createValue} required rows="4"></textarea>

						<label for="create-description">Description</label>
						<input id="create-description" bind:value={createDescription} />

						<label for="create-used-by">Used by (comma-separated)</label>
						<input id="create-used-by" bind:value={createUsedBy} />

						{#if createError}
							<p role="alert" class="error">{createError}</p>
						{/if}

						<div class="actions">
							<Dialog.Close type="button">Cancel</Dialog.Close>
							<button type="submit">Create</button>
						</div>
					</form>
				</Dialog.Content>
			</Dialog.Portal>
		</Dialog.Root>
	</header>

	{#if data.objects.length === 0}
		<p>No secrets stored yet.</p>
	{:else}
		<div class="table-scroll">
		<table>
			<thead>
				<tr>
					<th scope="col">Id</th>
					<th scope="col">Description</th>
					<th scope="col">Created</th>
					<th scope="col">Updated</th>
					<th scope="col">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each data.objects as object (object.id)}
					{@const attribution = data.attribution.get(object.id)}
					<tr>
						<td>{object.id}</td>
						<td>{object.description ?? ''}</td>
						<td>
							{#if attribution}
								{attribution.createdBy} &middot; {attribution.createdAt}
							{/if}
						</td>
						<td>
							{#if attribution}
								{attribution.updatedBy} &middot; {attribution.updatedAt}
							{/if}
						</td>
						<td class="row-actions">
							<button type="button" onclick={() => openView(object.id)}>View</button>
							<button type="button" onclick={() => openEdit(object.id)}>Edit</button>
							<button type="button" onclick={() => openDelete(object.id)}>Delete</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		</div>
	{/if}
</main>

<Dialog.Root bind:open={viewOpen}>
	<Dialog.Portal>
		<Dialog.Overlay class="overlay" />
		<Dialog.Content class="dialog">
			<Dialog.Title>{viewId}</Dialog.Title>
			<Dialog.Description>Sealed ciphertext, base64-encoded.</Dialog.Description>
			{#if viewError}
				<p role="alert" class="error">{viewError}</p>
			{:else}
				<textarea readonly rows="6" value={viewValue} aria-label="Ciphertext (base64)"
				></textarea>
			{/if}
			<div class="actions">
				<Dialog.Close type="button">Close</Dialog.Close>
			</div>
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>

<Dialog.Root bind:open={editOpen}>
	<Dialog.Portal>
		<Dialog.Overlay class="overlay" />
		<Dialog.Content class="dialog">
			<Dialog.Title>Edit {editId}</Dialog.Title>
			<form onsubmit={submitEdit}>
				<label for="edit-value">New ciphertext (base64)</label>
				<textarea id="edit-value" bind:value={editValue} required rows="6"></textarea>

				{#if editError}
					<p role="alert" class="error">{editError}</p>
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
			<AlertDialog.Title>Delete {deleteId}?</AlertDialog.Title>
			<AlertDialog.Description>
				This permanently removes the object. Anything still depending on it will start
				failing.
			</AlertDialog.Description>
			{#if deleteError}
				<p role="alert" class="error">{deleteError}</p>
			{/if}
			<div class="actions">
				<AlertDialog.Cancel type="button">Cancel</AlertDialog.Cancel>
				<AlertDialog.Action type="button" onclick={confirmDelete}>Delete</AlertDialog.Action>
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

	table {
		width: 100%;
		border-collapse: collapse;
	}

	th,
	td {
		text-align: left;
		padding: var(--space-2);
		border-bottom: 1px solid var(--color-border);
	}

	.row-actions button {
		margin-right: var(--space-2);
	}

	.error {
		color: var(--color-error);
	}

	:global(.overlay) {
		position: fixed;
		inset: 0;
		background: var(--color-overlay);
	}

	:global(.dialog) {
		position: fixed;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		background: var(--color-surface);
		color: var(--color-text);
		padding: var(--space-6);
		border-radius: 0.5rem;
		max-width: 32rem;
		width: 90vw;
		max-height: 85vh;
		overflow-y: auto;
	}

	form label {
		display: block;
		margin-top: 0.75rem;
	}

	form input,
	form textarea {
		width: 100%;
		box-sizing: border-box;
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1rem;
	}
</style>
