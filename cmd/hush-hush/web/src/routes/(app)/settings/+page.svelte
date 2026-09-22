<script lang="ts">
import { AlertDialog, Dialog } from 'bits-ui';
import { invalidate } from '$app/navigation';
import {
	ApiError,
	createToken,
	deleteCredential,
	renameCredential,
	revokeToken,
	type TokenWithValue,
} from '$lib/api';
import { registerPasskey } from '$lib/auth';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

function apiErrorMessage(err: unknown, fallback: string): string {
	return err instanceof ApiError ? err.message : fallback;
}

// Passkeys

let addOpen = $state(false);
let addNickname = $state('');
let addPending = $state(false);
let addError = $state('');

async function submitAdd(event: SubmitEvent) {
	event.preventDefault();
	addPending = true;
	addError = '';

	try {
		await registerPasskey(addNickname || undefined);
		addOpen = false;
		addNickname = '';
		await invalidate('app:settings');
	} catch (err) {
		addError = apiErrorMessage(err, 'Failed to register the passkey.');
	} finally {
		addPending = false;
	}
}

let renameOpen = $state(false);
let renameId = $state('');
let renameNickname = $state('');
let renameError = $state('');

function openRename(id: string, currentNickname: string | undefined) {
	renameId = id;
	renameNickname = currentNickname ?? '';
	renameError = '';
	renameOpen = true;
}

async function submitRename(event: SubmitEvent) {
	event.preventDefault();
	renameError = '';

	try {
		await renameCredential(renameId, renameNickname);
		renameOpen = false;
		await invalidate('app:settings');
	} catch (err) {
		renameError = apiErrorMessage(err, 'Failed to rename the passkey.');
	}
}

let deleteCredentialOpen = $state(false);
let deleteCredentialId = $state('');
let deleteCredentialError = $state('');

function openDeleteCredential(id: string) {
	deleteCredentialId = id;
	deleteCredentialError = '';
	deleteCredentialOpen = true;
}

async function confirmDeleteCredential() {
	deleteCredentialError = '';

	try {
		await deleteCredential(deleteCredentialId);
		deleteCredentialOpen = false;
		await invalidate('app:settings');
	} catch (err) {
		// web-ui/spec.md's own "last remaining credential" rule
		// (auth/spec.md's "Refusing to delete the last credential"
		// scenario) surfaces here as a plain 409 - shown in place
		// rather than crashing the page.
		deleteCredentialError = apiErrorMessage(
			err,
			'Failed to delete the passkey.',
		);
	}
}

// Tokens

let createTokenOpen = $state(false);
let tokenDescription = $state('');
let tokenTTLDays = $state(90);
let tokenError = $state('');
let createdToken: TokenWithValue | null = $state(null);

function resetTokenForm() {
	tokenDescription = '';
	tokenTTLDays = 90;
	tokenError = '';
	createdToken = null;
}

async function submitCreateToken(event: SubmitEvent) {
	event.preventDefault();
	tokenError = '';

	try {
		createdToken = await createToken(
			tokenDescription,
			tokenTTLDays * 24 * 60 * 60,
		);
		await invalidate('app:settings');
	} catch (err) {
		tokenError = apiErrorMessage(err, 'Failed to create the token.');
	}
}

function closeCreateToken() {
	createTokenOpen = false;
	resetTokenForm();
}

let revokeOpen = $state(false);
let revokeId = $state('');
let revokeError = $state('');

function openRevoke(id: string) {
	revokeId = id;
	revokeError = '';
	revokeOpen = true;
}

async function confirmRevoke() {
	revokeError = '';

	try {
		await revokeToken(revokeId);
		revokeOpen = false;
		await invalidate('app:settings');
	} catch (err) {
		revokeError = apiErrorMessage(err, 'Failed to revoke the token.');
	}
}
</script>

<svelte:head>
	<title>Settings - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-240 px-4">
	<h1>Settings</h1>

	<section class="mb-12">
		<header class="flex flex-wrap items-center justify-between gap-2">
			<h2>Passkeys</h2>
			<Dialog.Root bind:open={addOpen}>
				<Dialog.Trigger>Add passkey</Dialog.Trigger>
				<Dialog.Portal>
					<Dialog.Overlay class="overlay" />
					<Dialog.Content class="dialog">
						<Dialog.Title>Add a passkey</Dialog.Title>
						<form onsubmit={submitAdd}>
							<label class="mt-3 block" for="add-nickname">Nickname (optional)</label>
							<input id="add-nickname" class="w-full" bind:value={addNickname} />

							{#if addError}
								<p role="alert" class="text-error">{addError}</p>
							{/if}

							<div class="mt-4 flex justify-end gap-2">
								<Dialog.Close type="button">Cancel</Dialog.Close>
								<button type="submit" disabled={addPending}>
									{addPending ? 'Waiting for your browser…' : 'Register'}
								</button>
							</div>
						</form>
					</Dialog.Content>
				</Dialog.Portal>
			</Dialog.Root>
		</header>

		<table class="responsive-table">
			<thead>
				<tr>
					<th scope="col">Nickname</th>
					<th scope="col">Added</th>
					<th scope="col">Last used</th>
					<th scope="col">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each data.credentials as credential (credential.id)}
					<tr>
						<td data-label="Nickname">{credential.nickname ?? ''}</td>
						<td data-label="Added">{credential.created_at}</td>
						<td data-label="Last used">{credential.last_used_at ?? 'never'}</td>
						<td data-label="Actions" class="row-actions gap-2">
							<button
								type="button"
								onclick={() => openRename(credential.id, credential.nickname)}
							>
								Rename
							</button>
							<button
								type="button"
								class="danger"
								onclick={() => openDeleteCredential(credential.id)}
							>
								Delete
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</section>

	<section class="mb-12">
		<header class="flex flex-wrap items-center justify-between gap-2">
			<h2>Bearer tokens</h2>
			<Dialog.Root
				bind:open={createTokenOpen}
				onOpenChange={(open) => {
					if (!open) resetTokenForm();
				}}
			>
				<Dialog.Trigger>New token</Dialog.Trigger>
				<Dialog.Portal>
					<Dialog.Overlay class="overlay" />
					<Dialog.Content class="dialog">
						{#if createdToken}
							<Dialog.Title>Token created</Dialog.Title>
							<p role="alert" class="font-bold text-warning">
								This value is shown once. It will not be shown again - store it now.
							</p>
							<textarea
								readonly
								rows="3"
								value={createdToken.value}
								aria-label="Token value"
								class="w-full"
							></textarea>
							<div class="mt-4 flex justify-end gap-2">
								<button type="button" onclick={closeCreateToken}>Done</button>
							</div>
						{:else}
							<Dialog.Title>Create a token</Dialog.Title>
							<form onsubmit={submitCreateToken}>
								<label class="mt-3 block" for="token-description">Description</label>
								<input id="token-description" class="w-full" bind:value={tokenDescription} required />

								<label class="mt-3 block" for="token-ttl">Valid for (days)</label>
								<input
									id="token-ttl"
									class="w-full"
									type="number"
									min="1"
									bind:value={tokenTTLDays}
									required
								/>

								{#if tokenError}
									<p role="alert" class="text-error">{tokenError}</p>
								{/if}

								<div class="mt-4 flex justify-end gap-2">
									<Dialog.Close type="button">Cancel</Dialog.Close>
									<button type="submit">Create</button>
								</div>
							</form>
						{/if}
					</Dialog.Content>
				</Dialog.Portal>
			</Dialog.Root>
		</header>

		<table class="responsive-table">
			<thead>
				<tr>
					<th scope="col">Description</th>
					<th scope="col">Owner</th>
					<th scope="col">Created</th>
					<th scope="col">Expires</th>
					<th scope="col">Last used</th>
					<th scope="col">Status</th>
					<th scope="col">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each data.tokens as token (token.id)}
					<tr>
						<td data-label="Description">{token.description}</td>
						<td data-label="Owner">{token.owner ?? 'cli'}</td>
						<td data-label="Created">{token.created_at}</td>
						<td data-label="Expires">{token.expires_at}</td>
						<td data-label="Last used">{token.last_used_at ?? 'never'}</td>
						<td data-label="Status">{token.revoked ? 'Revoked' : 'Active'}</td>
						<td data-label="Actions" class="row-actions gap-2">
							{#if !token.revoked}
								<button type="button" class="danger" onclick={() => openRevoke(token.id)}>
									Revoke
								</button>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</section>
</main>

<Dialog.Root bind:open={renameOpen}>
	<Dialog.Portal>
		<Dialog.Overlay class="overlay" />
		<Dialog.Content class="dialog">
			<Dialog.Title>Rename passkey</Dialog.Title>
			<form onsubmit={submitRename}>
				<label class="mt-3 block" for="rename-nickname">Nickname</label>
				<input id="rename-nickname" class="w-full" bind:value={renameNickname} required />

				{#if renameError}
					<p role="alert" class="text-error">{renameError}</p>
				{/if}

				<div class="mt-4 flex justify-end gap-2">
					<Dialog.Close type="button">Cancel</Dialog.Close>
					<button type="submit">Save</button>
				</div>
			</form>
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>

<AlertDialog.Root bind:open={deleteCredentialOpen}>
	<AlertDialog.Portal>
		<AlertDialog.Overlay class="overlay" />
		<AlertDialog.Content class="dialog">
			<AlertDialog.Title>Delete this passkey?</AlertDialog.Title>
			<AlertDialog.Description>
				You won't be able to log in with it any more.
			</AlertDialog.Description>
			{#if deleteCredentialError}
				<p role="alert" class="text-error">{deleteCredentialError}</p>
			{/if}
			<div class="mt-4 flex justify-end gap-2">
				<AlertDialog.Cancel type="button">Cancel</AlertDialog.Cancel>
				<AlertDialog.Action type="button" onclick={confirmDeleteCredential}>
					Delete
				</AlertDialog.Action>
			</div>
		</AlertDialog.Content>
	</AlertDialog.Portal>
</AlertDialog.Root>

<AlertDialog.Root bind:open={revokeOpen}>
	<AlertDialog.Portal>
		<AlertDialog.Overlay class="overlay" />
		<AlertDialog.Content class="dialog">
			<AlertDialog.Title>Revoke this token?</AlertDialog.Title>
			<AlertDialog.Description>
				Anything still using it will stop being able to write.
			</AlertDialog.Description>
			{#if revokeError}
				<p role="alert" class="text-error">{revokeError}</p>
			{/if}
			<div class="mt-4 flex justify-end gap-2">
				<AlertDialog.Cancel type="button">Cancel</AlertDialog.Cancel>
				<AlertDialog.Action type="button" onclick={confirmRevoke}>Revoke</AlertDialog.Action>
			</div>
		</AlertDialog.Content>
	</AlertDialog.Portal>
</AlertDialog.Root>
