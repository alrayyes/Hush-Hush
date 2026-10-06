<script lang="ts">
import CheckIcon from '@lucide/svelte/icons/check';
import CopyIcon from '@lucide/svelte/icons/copy';
import { onDestroy } from 'svelte';
import { invalidate } from '$app/navigation';
import {
	ApiError,
	type ConsumerEntry,
	createObject,
	deleteObject,
	getObjectValue,
	type ObjectMetadata,
	updateObject,
} from '$lib/api';
import { TAGS_MAX_ITEMS } from '$lib/api-limits';
import { actorName } from '$lib/attribution';
import ConsumerCombobox from '$lib/ConsumerCombobox.svelte';
import { createCopier } from '$lib/clipboard';
import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
import { Checkbox } from '$lib/components/ui/checkbox/index.js';
import * as Dialog from '$lib/components/ui/dialog/index.js';
import { Input } from '$lib/components/ui/input/index.js';
import { Label } from '$lib/components/ui/label/index.js';
import { Textarea } from '$lib/components/ui/textarea/index.js';
import { consumersHref } from '$lib/consumers';
import { formatTimestamp } from '$lib/datetime';
import { resolveRecipients, sealValue } from '$lib/sealing';
import {
	filterByTag,
	formatTags,
	parseTags,
	tagCounts,
	validateTags,
} from '$lib/tags';
import { valueFieldAttributes } from '$lib/value-field';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

function apiErrorMessage(err: unknown, fallback: string): string {
	return err instanceof ApiError ? err.message : fallback;
}

let query = $state('');
let selectedTag: string | null = $state(null);
const tagPills = $derived(tagCounts(data.objects));
// A delete or retag can remove the last object carrying the selected tag;
// fall back to "All" rather than keep filtering on a pill that's gone.
const activeTag = $derived(
	selectedTag !== null && tagPills.some((p) => p.tag === selectedTag)
		? selectedTag
		: null,
);
const searchedObjects = $derived.by(() => {
	const needle = query.trim().toLowerCase();
	if (needle === '') return data.objects;
	return data.objects.filter(
		(o) =>
			o.slug.toLowerCase().includes(needle) ||
			(o.description ?? '').toLowerCase().includes(needle),
	);
});
const filteredObjects = $derived(filterByTag(searchedObjects, activeTag));

// Which card's copy button is showing its "Copied" state, and the timer
// that reverts it. One slot, so a newer copy replaces an older one.
let copiedId: string | null = $state(null);
const copier = createCopier<string>((id) => (copiedId = id));
onDestroy(copier.dispose);

// How many variants each name holds. A name with one reads as it always
// did; with several, each row and dialog says which consumers it is for.
const variantCounts = $derived(
	data.objects.reduce((counts, o) => {
		counts.set(o.slug, (counts.get(o.slug) ?? 0) + 1);
		return counts;
	}, new Map<string, number>()),
);

function variantLabel(object: ObjectMetadata): string {
	if ((variantCounts.get(object.slug) ?? 0) < 2) return '';
	const consumers = object.used_by ?? [];

	return consumers.length > 0
		? `Variant for ${consumers.join(', ')}`
		: 'Variant with no consumers';
}

let createOpen = $state(false);
let createError = $state('');
let createSlug = $state('');
let createValue = $state('');
let createDescription = $state('');
let createTags = $state('');
let createTagsError = $state('');
let createUsedBy: string[] = $state([]);
let createEntries: ConsumerEntry[] = $state([]);
// Opt-in, never sticky across opens - specs/secret-objects/spec.md's
// "Owner recipient is not the default" scenario (tasks.md's 5.2).
let createKeepReadableCopy = $state(false);

// Recomputed on every keystroke/selection so the "Create" button and its
// zero-recipient warning below track the current form state, not just
// its value at submit time.
const createRecipients = $derived(
	resolveRecipients(createUsedBy, createEntries),
);
// The recipients Value actually gets sealed to: every resolved consumer,
// plus the owner's own public key when the checkbox is checked - the
// checkbox alone can satisfy "at least one recipient" below, so a secret
// meant only for the owner never needs a placeholder consumer
// (specs/secret-objects/spec.md's "Opt-in owner-recipient inclusion at
// create time" requirement).
const createEffectiveRecipients = $derived(
	createKeepReadableCopy && data.ownerPublicKey
		? [...createRecipients.recipients, data.ownerPublicKey]
		: createRecipients.recipients,
);

function resetCreateForm() {
	createSlug = '';
	createValue = '';
	createDescription = '';
	createTags = '';
	createTagsError = '';
	createUsedBy = [];
	createKeepReadableCopy = false;
	createError = '';
}

async function submitCreate(event: SubmitEvent) {
	event.preventDefault();
	createError = '';

	const tags = parseTags(createTags);
	const tagsError = validateTags(tags);
	createTagsError = tagsError ?? '';
	if (tagsError) {
		return;
	}

	// A secret sealed to zero recipients could never be decrypted by
	// anyone - the create/edit modes this replaces at least produced
	// something a consumer's own tooling could read; this guards against
	// silently shipping something strictly worse
	// (openspec/changes/client-side-encryption/tasks.md's 4.3).
	if (createEffectiveRecipients.length === 0) {
		createError =
			'Add at least one consumer with a registered public key, or keep a readable copy for yourself, before creating this secret.';
		return;
	}

	try {
		const value = await sealValue(createValue, createEffectiveRecipients);
		await createObject({
			slug: createSlug,
			value,
			description: createDescription || undefined,
			tags: tags.length > 0 ? tags : undefined,
			used_by: createUsedBy.length > 0 ? createUsedBy : undefined,
			keep_readable_copy: createKeepReadableCopy || undefined,
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
// The variant being edited: a name can hold several (ADR 33), and the API
// only edits one when told which.
let editId = $state('');
// Only set when the name has several variants, so the dialog can say which.
let editVariant = $state('');
let editValue = $state('');
let editTags = $state('');
// What the field opened with, so an untouched field isn't sent at all.
let editInitialTags: string[] = [];
let editTagsError = $state('');
let editUsedBy: string[] = $state([]);
let editEntries: ConsumerEntry[] = $state([]);
let editError = $state('');
// Independent of whatever a previous create or update on this same
// object requested - an update reseals the whole value from scratch, and
// the server never persists this flag to read it back from
// (api/openapi.yaml's KeepReadableCopy schema).
let editKeepReadableCopy = $state(false);

const editRecipients = $derived(resolveRecipients(editUsedBy, editEntries));
const editEffectiveRecipients = $derived(
	editKeepReadableCopy && data.ownerPublicKey
		? [...editRecipients.recipients, data.ownerPublicKey]
		: editRecipients.recipients,
);

function openEdit(object: ObjectMetadata) {
	editSlug = object.slug;
	editId = object.id;
	editVariant = variantLabel(object);
	editValue = '';
	// Copied, not the same array reference data.objects holds - the
	// combobox mutates this in place as the user picks/adds consumers,
	// and canceling shouldn't leave that mutation sitting on data the
	// server never actually received.
	editUsedBy = [...(object.used_by ?? [])];
	editKeepReadableCopy = false;
	editError = '';
	editInitialTags = [...object.tags];
	editTags = formatTags(editInitialTags);
	editTagsError = '';
	editOpen = true;
}

async function submitEdit(event: SubmitEvent) {
	event.preventDefault();
	editError = '';

	const tags = parseTags(editTags);
	const tagsError = validateTags(tags);
	editTagsError = tagsError ?? '';
	if (tagsError) {
		return;
	}

	if (editEffectiveRecipients.length === 0) {
		editError =
			'Add at least one consumer with a registered public key, or keep a readable copy for yourself, before saving this secret.';
		return;
	}

	try {
		const value = await sealValue(editValue, editEffectiveRecipients);
		await updateObject(
			editSlug,
			value,
			editUsedBy,
			editKeepReadableCopy,
			// Omitted when the field is as it opened, which leaves the tags alone.
			tags.join(',') === editInitialTags.join(',') ? undefined : tags,
			editId,
		);
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
let viewVariant = $state('');
let viewValue = $state('');
let viewUsedBy: string[] = $state([]);
let viewError = $state('');

async function openView(object: ObjectMetadata) {
	viewSlug = object.slug;
	viewVariant = variantLabel(object);
	viewValue = '';
	viewUsedBy = object.used_by ?? [];
	viewError = '';
	viewOpen = true;

	try {
		viewValue = await getObjectValue(object.slug, object.id);
	} catch (err) {
		viewError = apiErrorMessage(err, 'Failed to fetch the secret.');
	}
}

let deleteOpen = $state(false);
let deleteSlug = $state('');
let deleteId = $state('');
let deleteVariant = $state('');
let deleteError = $state('');

function openDelete(object: ObjectMetadata) {
	deleteSlug = object.slug;
	deleteId = object.id;
	deleteVariant = variantLabel(object);
	deleteError = '';
	deleteOpen = true;
}

async function confirmDelete() {
	deleteError = '';

	try {
		await deleteObject(deleteSlug, deleteId);
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
					<div class="space-y-4">
						<div class="space-y-1">
							<Label for="create-id">Id</Label>
							<Input id="create-id" class="w-full" bind:value={createSlug} required />
						</div>

						<div class="space-y-1">
							<Label for="create-value">Value</Label>
							<Textarea
								id="create-value"
								class="w-full"
								bind:value={createValue}
								{...valueFieldAttributes}
								required
								rows={4}
							/>
						</div>

						<div class="space-y-1">
							<Label for="create-description">Description</Label>
							<Input id="create-description" class="w-full" bind:value={createDescription} />
						</div>

						<div class="space-y-1">
							<Label for="create-tags">Tags</Label>
							<Input
								id="create-tags"
								class="w-full"
								bind:value={createTags}
								oninput={() => (createTagsError = '')}
								aria-invalid={createTagsError ? true : undefined}
								aria-describedby={createTagsError ? 'create-tags-hint create-tags-error' : 'create-tags-hint'}
							/>
							<p id="create-tags-hint" class="text-sm text-text-muted">
								Optional. Separate with commas, up to {TAGS_MAX_ITEMS}. Lowercase
								letters, numbers and . _ / - only.
							</p>
							{#if createTagsError}
								<p id="create-tags-error" role="alert" class="text-sm text-error">
									{createTagsError}
								</p>
							{/if}
						</div>

						<div class="space-y-1">
							<Label for="create-used-by">Used by</Label>
							<ConsumerCombobox
								id="create-used-by"
								bind:value={createUsedBy}
								bind:entries={createEntries}
							/>
							<p class="text-sm text-text-muted">
								Sealed in your browser with age, to the registered public keys
								of the consumers selected above - never sent in the clear.
							</p>
						</div>

						{#if data.ownerPublicKey}
							<div class="flex items-center gap-2">
								<Checkbox
									id="create-keep-readable-copy"
									bind:checked={createKeepReadableCopy}
								/>
								<Label for="create-keep-readable-copy">
									Keep a readable copy for yourself
								</Label>
							</div>
						{/if}

						{#if createEffectiveRecipients.length === 0}
							<p role="alert" class="font-bold text-warning">
								Add at least one consumer with a registered public key, or keep
								a readable copy for yourself - a secret sealed to nobody could
								never be decrypted.
							</p>
						{/if}

						{#if createError}
							<p role="alert" class="text-error">{createError}</p>
						{/if}
					</div>

					<Dialog.Footer>
						<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
							Cancel
						</Dialog.Close>
						<Button type="submit" disabled={createEffectiveRecipients.length === 0}>
							Create
						</Button>
					</Dialog.Footer>
				</form>
			</Dialog.Content>
		</Dialog.Root>
	</header>

	{#if data.usedByFilter}
		<ul class="mb-4 flex list-none flex-wrap gap-2 p-0">
			<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
				consumer: {data.usedByFilter}
				<Button
					variant="ghost"
					size="icon-xs"
					class="ml-1 h-auto w-auto p-0"
					href="/"
					aria-label="Clear consumer filter"
				>
					&times;
				</Button>
			</li>
		</ul>
	{/if}

	{#if data.objects.length === 0}
		<p>No secrets stored yet.</p>
	{:else}
		<div class="sticky top-0 z-10 bg-background py-2">
			<Input
				type="search"
				class="w-full"
				aria-label="Filter secrets"
				placeholder="Filter by id or description"
				bind:value={query}
			/>
			{#if tagPills.length > 0}
				<div role="group" aria-label="Filter by tag" class="mt-2 flex flex-wrap gap-2">
					<Button
						type="button"
						size="sm"
						class="min-h-11 rounded-full"
						variant={activeTag === null ? 'default' : 'outline'}
						aria-pressed={activeTag === null}
						onclick={() => (selectedTag = null)}
					>
						All ({data.objects.length})
					</Button>
					{#each tagPills as pill (pill.tag)}
						{@const pressed = activeTag === pill.tag}
						<Button
							type="button"
							size="sm"
							class="min-h-11 rounded-full"
							variant={pressed ? 'default' : 'outline'}
							aria-pressed={pressed}
							onclick={() => (selectedTag = pill.tag)}
						>
							{pill.tag} ({pill.count})
						</Button>
					{/each}
				</div>
			{/if}
		</div>

		{#if filteredObjects.length === 0}
			<p>No secrets match &ldquo;{query}&rdquo;.</p>
		{/if}

		<ul aria-label="Secrets" class="m-0 list-none space-y-3 p-0 md:hidden">
			{#each filteredObjects as object (object.id)}
				<li class="space-y-2 rounded-lg border border-border bg-background p-4">
					<div class="flex items-start justify-between gap-2">
						<span class="min-w-0 break-all font-mono text-sm font-semibold">{object.slug}</span>
						<Button
							variant="outline"
							class="min-h-11 min-w-11"
							aria-label={copiedId === object.id
								? `Copied ${object.slug}`
								: `Copy slug ${object.slug}`}
							onclick={() => copier.copy(object.id, object.slug)}
						>
							{#if copiedId === object.id}
								<CheckIcon aria-hidden="true" />
							{:else}
								<CopyIcon aria-hidden="true" />
							{/if}
							{copiedId === object.id ? 'Copied' : 'Copy'}
						</Button>
					</div>
					{#if object.description}
						<p class="m-0 text-sm">{object.description}</p>
					{/if}
					{#if object.used_by?.length}
						<p class="m-0 text-sm">
							<span class="text-text-muted">Used by</span>
							<span class="font-mono">{object.used_by.join(', ')}</span>
						</p>
					{/if}
					{#if object.tags.length > 0}
						<div class="flex flex-wrap gap-1">
							{#each object.tags as tag (tag)}
								<span
									data-testid="tag"
									class="rounded-full border border-border px-2 py-0.5 text-xs text-text-muted"
									>{tag}</span
								>
							{/each}
						</div>
					{/if}
					<p class="m-0">
						<span class="rounded-2xl bg-border-subtle px-2 py-1 text-xs">age-encrypted (X25519)</span>
					</p>
					{#if object.created_at}
						<p class="m-0 text-sm text-text-muted">
							Created
							<time datetime={object.created_at} title={object.created_at}>
								{formatTimestamp(object.created_at)}
							</time>
							{#if object.created_by}
								by {actorName(object.created_by)}
							{/if}
						</p>
					{/if}
					{#if object.updated_at}
						<p class="m-0 text-sm text-text-muted">
							Updated
							<time datetime={object.updated_at} title={object.updated_at}>
								{formatTimestamp(object.updated_at)}
							</time>
							{#if object.updated_by}
								by {actorName(object.updated_by)}
							{/if}
						</p>
					{/if}
					<div class="flex flex-wrap gap-2">
						<Button variant="outline" class="min-h-11 min-w-11" onclick={() => openView(object)}>
							View
						</Button>
						<Button
							href={`/secrets/${encodeURIComponent(object.slug)}?id=${encodeURIComponent(object.id)}`}
							variant="outline"
							class="min-h-11 min-w-11"
						>
							Inspect
						</Button>
						<Button variant="outline" class="min-h-11 min-w-11" onclick={() => openEdit(object)}>
							Edit
						</Button>
						<Button
							variant="destructive"
							class="min-h-11 min-w-11"
							onclick={() => openDelete(object)}
						>
							Delete
						</Button>
					</div>
				</li>
			{/each}
		</ul>

		<table class="responsive-table hidden md:table">
			<thead>
				<tr>
					<th scope="col" class="px-4 py-3">Id</th>
					<th scope="col" class="px-4 py-3">Description</th>
					<th scope="col" class="px-4 py-3">Tags</th>
					<th scope="col" class="px-4 py-3">Created by</th>
					<th scope="col" class="px-4 py-3">Created</th>
					<th scope="col" class="px-4 py-3">Updated by</th>
					<th scope="col" class="px-4 py-3">Updated</th>
					<th scope="col" class="px-4 py-3">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each filteredObjects as object (object.id)}
					<tr>
						<td data-label="Id">
							{object.slug}
							{#if object.used_by?.length}
								<span class="block max-w-40 break-words font-mono text-xs text-text-muted">
									{object.used_by.join(', ')}
								</span>
							{/if}
						</td>
						<td data-label="Description">
							<span class="block max-w-40 truncate" title={object.description ?? ''}>
								{object.description ?? ''}
							</span>
						</td>
						<td data-label="Tags">
							{#if object.tags.length > 0}
								<div class="flex max-w-28 flex-wrap gap-1">
									{#each object.tags as tag (tag)}
										<span
											data-testid="tag"
											class="rounded-full border border-border px-2 py-0.5 text-xs text-text-muted"
											>{tag}</span
										>
									{/each}
								</div>
							{/if}
						</td>
						<td data-label="Created by">{object.created_by ? actorName(object.created_by) : ''}</td>
						<td data-label="Created">
							{#if object.created_at}
								<time datetime={object.created_at} title={object.created_at}>
									{formatTimestamp(object.created_at)}
								</time>
							{/if}
						</td>
						<td data-label="Updated by">{object.updated_by ? actorName(object.updated_by) : ''}</td>
						<td data-label="Updated">
							{#if object.updated_at}
								<time datetime={object.updated_at} title={object.updated_at}>
									{formatTimestamp(object.updated_at)}
								</time>
							{/if}
						</td>
						<!-- The card list above offers the same actions; e2e/layout-parity.ts
							fails if the two drift (alrayyes/hush-hush#577). -->
						<td data-label="Actions" class="row-actions min-w-44 gap-3">
							<Button
								variant="outline"
								size="sm"
								aria-label={copiedId === object.id
									? `Copied ${object.slug}`
									: `Copy slug ${object.slug}`}
								onclick={() => copier.copy(object.id, object.slug)}
							>
								{#if copiedId === object.id}
									<CheckIcon aria-hidden="true" />
								{:else}
									<CopyIcon aria-hidden="true" />
								{/if}
								<span class="sr-only">{copiedId === object.id ? 'Copied' : 'Copy'}</span>
							</Button>
							<Button
								href={`/secrets/${encodeURIComponent(object.slug)}?id=${encodeURIComponent(object.id)}`}
								variant="outline"
								size="sm"
							>
								Inspect
							</Button>
							<Button variant="outline" size="sm" onclick={() => openView(object)}>
								View
							</Button>
							<Button variant="outline" size="sm" onclick={() => openEdit(object)}>
								Edit
							</Button>
							<Button
								variant="destructive"
								size="sm"
								onclick={() => openDelete(object)}
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
			<Dialog.Description>
				{viewVariant ? `${viewVariant}. ` : ''}Sealed ciphertext, base64-encoded.
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4">
			{#if viewError}
				<p role="alert" class="text-error">{viewError}</p>
			{:else}
				<Textarea readonly rows={6} value={viewValue} aria-label="Ciphertext (base64)" />
			{/if}
			<div class="space-y-1">
				<p class="font-bold">Used by</p>
				{#if viewUsedBy.length > 0}
					<ul class="m-0 space-y-1 pl-5">
						{#each viewUsedBy as consumer (consumer)}
							<li><a href={consumersHref(1, consumer)}>{consumer}</a></li>
						{/each}
					</ul>
				{:else}
					<p>No recorded consumers.</p>
				{/if}
			</div>
		</div>
	</Dialog.Content>
</Dialog.Root>

<Dialog.Root bind:open={editOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Edit {editSlug}</Dialog.Title>
			{#if editVariant}
				<Dialog.Description>{editVariant}.</Dialog.Description>
			{/if}
		</Dialog.Header>
		<form onsubmit={submitEdit}>
			<div class="space-y-4">
				<div class="space-y-1">
					<Label for="edit-value">New value</Label>
					<Textarea
						id="edit-value"
						class="w-full"
						bind:value={editValue}
						required
						rows={6}
						{...valueFieldAttributes}
					/>
				</div>

				<div class="space-y-1">
					<Label for="edit-tags">Tags</Label>
					<Input
						id="edit-tags"
						class="w-full"
						bind:value={editTags}
						oninput={() => (editTagsError = '')}
						aria-invalid={editTagsError ? true : undefined}
						aria-describedby={editTagsError ? 'edit-tags-hint edit-tags-error' : 'edit-tags-hint'}
					/>
					<p id="edit-tags-hint" class="text-sm text-text-muted">
						Optional. Separate with commas, up to {TAGS_MAX_ITEMS}. Lowercase
						letters, numbers and . _ / - only.
					</p>
					{#if editTagsError}
						<p id="edit-tags-error" role="alert" class="text-sm text-error">
							{editTagsError}
						</p>
					{/if}
				</div>

				<div class="space-y-1">
					<Label for="edit-used-by">Used by</Label>
					<ConsumerCombobox
						id="edit-used-by"
						bind:value={editUsedBy}
						bind:entries={editEntries}
					/>
					<p class="text-sm text-text-muted">
						Sealed in your browser with age, to the registered public keys of
						the consumers selected above - never sent in the clear.
					</p>
				</div>

				{#if data.ownerPublicKey}
					<div class="flex items-center gap-2">
						<Checkbox id="edit-keep-readable-copy" bind:checked={editKeepReadableCopy} />
						<Label for="edit-keep-readable-copy">
							Keep a readable copy for yourself
						</Label>
					</div>
				{/if}

				{#if editEffectiveRecipients.length === 0}
					<p role="alert" class="font-bold text-warning">
						Add at least one consumer with a registered public key, or keep a
						readable copy for yourself - a secret sealed to nobody could never
						be decrypted.
					</p>
				{/if}

				{#if editError}
					<p role="alert" class="text-error">{editError}</p>
				{/if}
			</div>

			<Dialog.Footer>
				<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
					Cancel
				</Dialog.Close>
				<Button type="submit" disabled={editEffectiveRecipients.length === 0}>
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
				{deleteVariant ? `${deleteVariant}. ` : ''}This permanently removes the object. Anything still depending on it will start
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
