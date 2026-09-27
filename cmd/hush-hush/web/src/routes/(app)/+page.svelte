<script lang="ts">
import { invalidate } from '$app/navigation';
import {
	ApiError,
	type ConsumerEntry,
	createObject,
	deleteObject,
	getObjectValue,
	updateObject,
} from '$lib/api';
import ConsumerCombobox from '$lib/ConsumerCombobox.svelte';
import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
import * as Dialog from '$lib/components/ui/dialog/index.js';
import { Input } from '$lib/components/ui/input/index.js';
import { Label } from '$lib/components/ui/label/index.js';
import { Textarea } from '$lib/components/ui/textarea/index.js';
import { resolveRecipients, sealValue } from '$lib/sealing';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

function apiErrorMessage(err: unknown, fallback: string): string {
	return err instanceof ApiError ? err.message : fallback;
}

let createOpen = $state(false);
let createError = $state('');
let createSlug = $state('');
let createValue = $state('');
let createDescription = $state('');
let createUsedBy: string[] = $state([]);
let createEntries: ConsumerEntry[] = $state([]);

// Recomputed on every keystroke/selection so the "Create" button and its
// zero-recipient warning below track the current form state, not just
// its value at submit time.
const createRecipients = $derived(
	resolveRecipients(createUsedBy, createEntries),
);

function resetCreateForm() {
	createSlug = '';
	createValue = '';
	createDescription = '';
	createUsedBy = [];
	createError = '';
}

async function submitCreate(event: SubmitEvent) {
	event.preventDefault();
	createError = '';

	// A secret sealed to zero recipients could never be decrypted by
	// anyone - the create/edit modes this replaces at least produced
	// something a consumer's own tooling could read; this guards against
	// silently shipping something strictly worse
	// (openspec/changes/client-side-encryption/tasks.md's 4.3).
	if (createRecipients.recipients.length === 0) {
		createError =
			'Add at least one consumer with a registered public key before creating this secret.';
		return;
	}

	try {
		const value = await sealValue(createValue, createRecipients.recipients);
		await createObject({
			slug: createSlug,
			value,
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
let editSlug = $state('');
let editValue = $state('');
let editUsedBy: string[] = $state([]);
let editEntries: ConsumerEntry[] = $state([]);
let editError = $state('');

const editRecipients = $derived(resolveRecipients(editUsedBy, editEntries));

function openEdit(slug: string) {
	editSlug = slug;
	editValue = '';
	// Copied, not the same array reference data.objects holds - the
	// combobox mutates this in place as the user picks/adds consumers,
	// and canceling shouldn't leave that mutation sitting on data the
	// server never actually received.
	editUsedBy = [...(data.objects.find((o) => o.slug === slug)?.used_by ?? [])];
	editError = '';
	editOpen = true;
}

async function submitEdit(event: SubmitEvent) {
	event.preventDefault();
	editError = '';

	if (editRecipients.recipients.length === 0) {
		editError =
			'Add at least one consumer with a registered public key before saving this secret.';
		return;
	}

	try {
		const value = await sealValue(editValue, editRecipients.recipients);
		await updateObject(editSlug, value, editUsedBy);
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
let viewSlug = $state('');
let viewValue = $state('');
let viewUsedBy: string[] = $state([]);
let viewError = $state('');

async function openView(slug: string) {
	viewSlug = slug;
	viewValue = '';
	viewUsedBy = data.objects.find((o) => o.slug === slug)?.used_by ?? [];
	viewError = '';
	viewOpen = true;

	try {
		viewValue = await getObjectValue(slug);
	} catch (err) {
		viewError = apiErrorMessage(err, 'Failed to fetch the secret.');
	}
}

let deleteOpen = $state(false);
let deleteSlug = $state('');
let deleteError = $state('');

function openDelete(slug: string) {
	deleteSlug = slug;
	deleteError = '';
	deleteOpen = true;
}

async function confirmDelete() {
	deleteError = '';

	try {
		await deleteObject(deleteSlug);
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
			<Dialog.Trigger class={buttonVariants({ variant: 'default' })}>
				New secret
			</Dialog.Trigger>
			<Dialog.Content>
				<Dialog.Header>
					<Dialog.Title>Create a secret</Dialog.Title>
				</Dialog.Header>
				<form onsubmit={submitCreate}>
					<Label for="create-id">Id</Label>
					<Input id="create-id" class="mt-1 mb-3 w-full" bind:value={createSlug} required />

					<Label for="create-value">Value</Label>
					<Textarea
						id="create-value"
						class="mt-1 mb-3 w-full"
						bind:value={createValue}
						required
						rows={4}
					/>

					<Label for="create-description">Description</Label>
					<Input
						id="create-description"
						class="mt-1 mb-3 w-full"
						bind:value={createDescription}
					/>

					<Label for="create-used-by">Used by</Label>
					<ConsumerCombobox
						id="create-used-by"
						bind:value={createUsedBy}
						bind:entries={createEntries}
					/>
					<p class="mt-1 mb-3 text-sm">
						Sealed in your browser with age, to the registered public keys of
						the consumers selected above - never sent in the clear.
					</p>

					{#if createRecipients.recipients.length === 0}
						<p role="alert" class="mb-3 font-bold text-warning">
							Add at least one consumer with a registered public key - a
							secret sealed to nobody could never be decrypted.
						</p>
					{/if}

					{#if createError}
						<p role="alert" class="text-error">{createError}</p>
					{/if}

					<Dialog.Footer>
						<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
							Cancel
						</Dialog.Close>
						<Button type="submit" disabled={createRecipients.recipients.length === 0}>
							Create
						</Button>
					</Dialog.Footer>
				</form>
			</Dialog.Content>
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
				{#each data.objects as object (object.slug)}
					{@const attribution = data.attribution.get(object.slug)}
					<tr>
						<td data-label="Id">{object.slug}</td>
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
							<Button variant="outline" size="sm" onclick={() => openView(object.slug)}>
								View
							</Button>
							<Button variant="outline" size="sm" onclick={() => openEdit(object.slug)}>
								Edit
							</Button>
							<Button
								variant="destructive"
								size="sm"
								onclick={() => openDelete(object.slug)}
							>
								Delete
							</Button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</main>

<Dialog.Root bind:open={viewOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>{viewSlug}</Dialog.Title>
			<Dialog.Description>Sealed ciphertext, base64-encoded.</Dialog.Description>
		</Dialog.Header>
		{#if viewError}
			<p role="alert" class="text-error">{viewError}</p>
		{:else}
			<Textarea readonly rows={6} value={viewValue} aria-label="Ciphertext (base64)" />
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
	</Dialog.Content>
</Dialog.Root>

<Dialog.Root bind:open={editOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Edit {editSlug}</Dialog.Title>
		</Dialog.Header>
		<form onsubmit={submitEdit}>
			<Label for="edit-value">New value</Label>
			<Textarea
				id="edit-value"
				class="mt-1 mb-3 w-full"
				bind:value={editValue}
				required
				rows={6}
			/>

			<Label for="edit-used-by">Used by</Label>
			<ConsumerCombobox
				id="edit-used-by"
				bind:value={editUsedBy}
				bind:entries={editEntries}
			/>
			<p class="mt-1 mb-3 text-sm">
				Sealed in your browser with age, to the registered public keys of
				the consumers selected above - never sent in the clear.
			</p>

			{#if editRecipients.recipients.length === 0}
				<p role="alert" class="mb-3 font-bold text-warning">
					Add at least one consumer with a registered public key - a secret
					sealed to nobody could never be decrypted.
				</p>
			{/if}

			{#if editError}
				<p role="alert" class="text-error">{editError}</p>
			{/if}

			<Dialog.Footer>
				<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
					Cancel
				</Dialog.Close>
				<Button type="submit" disabled={editRecipients.recipients.length === 0}>
					Save
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<AlertDialog.Root bind:open={deleteOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete {deleteSlug}?</AlertDialog.Title>
			<AlertDialog.Description>
				This permanently removes the object. Anything still depending on it will start
				failing.
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
