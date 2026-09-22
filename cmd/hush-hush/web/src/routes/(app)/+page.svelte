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
		// A create can introduce a consumer the directory page hasn't seen
		// yet, or add to an existing one's count.
		await invalidate('app:consumers');
	} catch (err) {
		createError = apiErrorMessage(err, 'Failed to create the secret.');
	}
}

let editOpen = $state(false);
let editId = $state('');
let editValue = $state('');
let editUsedBy: string[] = $state([]);
let editError = $state('');

function openEdit(id: string) {
	editId = id;
	editValue = '';
	// Copied, not the same array reference data.objects holds - the
	// combobox mutates this in place as the user picks/adds consumers,
	// and canceling shouldn't leave that mutation sitting on data the
	// server never actually received.
	editUsedBy = [...(data.objects.find((o) => o.id === id)?.used_by ?? [])];
	editError = '';
	editOpen = true;
}

async function submitEdit(event: SubmitEvent) {
	event.preventDefault();
	editError = '';

	try {
		await updateObject(editId, editValue, editUsedBy);
		editOpen = false;
		await invalidate('app:objects');
		// An edit can introduce a consumer the directory page hasn't seen
		// yet, or drop the last secret recording an existing one.
		await invalidate('app:consumers');
	} catch (err) {
		editError = apiErrorMessage(err, 'Failed to update the secret.');
	}
}

let viewOpen = $state(false);
let viewId = $state('');
let viewValue = $state('');
let viewUsedBy: string[] = $state([]);
let viewError = $state('');

async function openView(id: string) {
	viewId = id;
	viewValue = '';
	viewUsedBy = data.objects.find((o) => o.id === id)?.used_by ?? [];
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
		// A delete can remove a consumer's last secret, dropping it from
		// the directory entirely, or lower its count.
		await invalidate('app:consumers');
	} catch (err) {
		deleteError = apiErrorMessage(err, 'Failed to delete the secret.');
	}
}
</script>

<svelte:head>
	<title>Secrets - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-240 px-4">
	<header class="flex flex-wrap items-center justify-between gap-2">
		<h1>Secrets</h1>
		<Dialog.Root bind:open={createOpen}>
			<Dialog.Trigger>New secret</Dialog.Trigger>
			<Dialog.Portal>
				<Dialog.Overlay class="overlay" />
				<Dialog.Content class="dialog">
					<Dialog.Title>Create a secret</Dialog.Title>
					<form onsubmit={submitCreate}>
						<label class="mt-3 block" for="create-id">Id</label>
						<input id="create-id" class="w-full" bind:value={createId} required />

						<label class="mt-3 block" for="create-value">
							{createPlainText ? 'Value (plain text)' : 'Ciphertext (base64)'}
						</label>
						<textarea id="create-value" class="w-full" bind:value={createValue} required rows="4"
						></textarea>

						<label class="mt-3 flex items-center gap-2">
							<input type="checkbox" bind:checked={createPlainText} />
							Plain text - base64-encoded for you, <strong>not encrypted</strong>
						</label>

						<label class="mt-3 block" for="create-description">Description</label>
						<input id="create-description" class="w-full" bind:value={createDescription} />

						<label class="mt-3 block" for="create-used-by">Used by</label>
						<ConsumerCombobox id="create-used-by" bind:value={createUsedBy} />

						{#if createError}
							<p role="alert" class="text-error">{createError}</p>
						{/if}

						<div class="mt-4 flex justify-end gap-2">
							<Dialog.Close type="button">Cancel</Dialog.Close>
							<button type="submit">Create</button>
						</div>
					</form>
				</Dialog.Content>
			</Dialog.Portal>
		</Dialog.Root>
	</header>

	{#if data.usedByFilter}
		<p class="flex flex-wrap items-center gap-2">
			Filtered to consumer <strong>{data.usedByFilter}</strong>
			<a href="/">Clear filter</a>
		</p>
	{/if}

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
						<td data-label="Actions" class="row-actions gap-2">
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
				<p role="alert" class="text-error">{viewError}</p>
			{:else}
				<textarea readonly rows="6" value={viewValue} aria-label="Ciphertext (base64)"
				></textarea>
			{/if}
			<p class="mt-3 font-bold">Used by</p>
			{#if viewUsedBy.length > 0}
				<ul class="m-0 pl-5">
					{#each viewUsedBy as consumer (consumer)}
						<li>{consumer}</li>
					{/each}
				</ul>
			{:else}
				<p>No recorded consumers.</p>
			{/if}
			<div class="mt-4 flex justify-end gap-2">
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
				<label class="mt-3 block" for="edit-value">New ciphertext (base64)</label>
				<textarea id="edit-value" class="w-full" bind:value={editValue} required rows="6"
				></textarea>

				<label class="mt-3 block" for="edit-used-by">Used by</label>
				<ConsumerCombobox id="edit-used-by" bind:value={editUsedBy} />

				{#if editError}
					<p role="alert" class="text-error">{editError}</p>
				{/if}

				<div class="mt-4 flex justify-end gap-2">
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
				<p role="alert" class="text-error">{deleteError}</p>
			{/if}
			<div class="mt-4 flex justify-end gap-2">
				<AlertDialog.Cancel type="button">Cancel</AlertDialog.Cancel>
				<AlertDialog.Action type="button" class="danger" onclick={confirmDelete}>
					Delete
				</AlertDialog.Action>
			</div>
		</AlertDialog.Content>
	</AlertDialog.Portal>
</AlertDialog.Root>
