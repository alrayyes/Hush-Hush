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
import ConsumerCombobox from '$lib/ConsumerCombobox.svelte';
import { utf8ToBase64 } from '$lib/encoding';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

function apiErrorMessage(err: unknown, fallback: string): string {
	return err instanceof ApiError ? err.message : fallback;
}

let createOpen = $state(false);
let createError = $state('');
let createId = $state('');
let createValue = $state('');
let createPlainText = $state(false);
let createDescription = $state('');
let createUsedBy: string[] = $state([]);

function resetCreateForm() {
	createId = '';
	createValue = '';
	createPlainText = false;
	createDescription = '';
	createUsedBy = [];
	createError = '';
}

async function submitCreate(event: SubmitEvent) {
	event.preventDefault();
	createError = '';

	try {
		await createObject({
			id: createId,
			// createPlainText is an explicit, off-by-default opt-in - the
			// field otherwise means exactly what its label says, already-
			// sealed ciphertext, sent as-is. Base64 alone is an encoding,
			// not encryption: this mode stores a trivially-reversible
			// obfuscation of whatever's typed, not a real secret the
			// server can't read (alrayyes/hush-hush#268).
			value: createPlainText ? utf8ToBase64(createValue) : createValue,
			description: createDescription || undefined,
			used_by: createUsedBy.length > 0 ? createUsedBy : undefined,
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

						<label for="create-value">
							{createPlainText ? 'Value (plain text)' : 'Ciphertext (base64)'}
						</label>
						<textarea id="create-value" bind:value={createValue} required rows="4"></textarea>

						<label class="checkbox">
							<input type="checkbox" bind:checked={createPlainText} />
							Plain text - base64-encoded for you, <strong>not encrypted</strong>
						</label>

						<label for="create-description">Description</label>
						<input id="create-description" bind:value={createDescription} />

						<label for="create-used-by">Used by</label>
						<ConsumerCombobox id="create-used-by" bind:value={createUsedBy} />

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
		<table class="responsive-table">
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
						<td data-label="Id">{object.id}</td>
						<td data-label="Description">{object.description ?? ''}</td>
						<td data-label="Created">
							{#if attribution}
								{attribution.createdBy} &middot; {attribution.createdAt}
							{/if}
						</td>
						<td data-label="Updated">
							{#if attribution}
								{attribution.updatedBy} &middot; {attribution.updatedAt}
							{/if}
						</td>
						<td data-label="Actions" class="row-actions">
							<button type="button" onclick={() => openView(object.id)}>View</button>
							<button type="button" onclick={() => openEdit(object.id)}>Edit</button>
							<button type="button" class="danger" onclick={() => openDelete(object.id)}>
								Delete
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
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
				<AlertDialog.Action type="button" class="danger" onclick={confirmDelete}>
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

	.row-actions button {
		margin-right: var(--space-2);
	}

	.error {
		color: var(--color-error);
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

	.checkbox {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.checkbox input {
		width: auto;
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1rem;
	}
</style>
