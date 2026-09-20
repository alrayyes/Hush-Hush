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

<main>
	<h1>Settings</h1>

	<section>
		<header class="section-header">
			<h2>Passkeys</h2>
			<Dialog.Root bind:open={addOpen}>
				<Dialog.Trigger>Add passkey</Dialog.Trigger>
				<Dialog.Portal>
					<Dialog.Overlay class="overlay" />
					<Dialog.Content class="dialog">
						<Dialog.Title>Add a passkey</Dialog.Title>
						<form onsubmit={submitAdd}>
							<label for="add-nickname">Nickname (optional)</label>
							<input id="add-nickname" bind:value={addNickname} />

							{#if addError}
								<p role="alert" class="error">{addError}</p>
							{/if}

							<div class="actions">
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

		<div class="table-scroll">
			<table>
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
							<td>{credential.nickname ?? ''}</td>
							<td>{credential.created_at}</td>
							<td>{credential.last_used_at ?? 'never'}</td>
							<td class="row-actions">
								<button
									type="button"
									onclick={() => openRename(credential.id, credential.nickname)}
								>
									Rename
								</button>
								<button type="button" onclick={() => openDeleteCredential(credential.id)}>
									Delete
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>

	<section>
		<header class="section-header">
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
							<p role="alert" class="warning">
								This value is shown once. It will not be shown again - store it now.
							</p>
							<textarea readonly rows="3" value={createdToken.value} aria-label="Token value"
							></textarea>
							<div class="actions">
								<button type="button" onclick={closeCreateToken}>Done</button>
							</div>
						{:else}
							<Dialog.Title>Create a token</Dialog.Title>
							<form onsubmit={submitCreateToken}>
								<label for="token-description">Description</label>
								<input id="token-description" bind:value={tokenDescription} required />

								<label for="token-ttl">Valid for (days)</label>
								<input
									id="token-ttl"
									type="number"
									min="1"
									bind:value={tokenTTLDays}
									required
								/>

								{#if tokenError}
									<p role="alert" class="error">{tokenError}</p>
								{/if}

								<div class="actions">
									<Dialog.Close type="button">Cancel</Dialog.Close>
									<button type="submit">Create</button>
								</div>
							</form>
						{/if}
					</Dialog.Content>
				</Dialog.Portal>
			</Dialog.Root>
		</header>

		<div class="table-scroll">
			<table>
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
							<td>{token.description}</td>
							<td>{token.owner ?? 'cli'}</td>
							<td>{token.created_at}</td>
							<td>{token.expires_at}</td>
							<td>{token.last_used_at ?? 'never'}</td>
							<td>{token.revoked ? 'Revoked' : 'Active'}</td>
							<td class="row-actions">
								{#if !token.revoked}
									<button type="button" onclick={() => openRevoke(token.id)}>Revoke</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>
</main>

<Dialog.Root bind:open={renameOpen}>
	<Dialog.Portal>
		<Dialog.Overlay class="overlay" />
		<Dialog.Content class="dialog">
			<Dialog.Title>Rename passkey</Dialog.Title>
			<form onsubmit={submitRename}>
				<label for="rename-nickname">Nickname</label>
				<input id="rename-nickname" bind:value={renameNickname} required />

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

<AlertDialog.Root bind:open={deleteCredentialOpen}>
	<AlertDialog.Portal>
		<AlertDialog.Overlay class="overlay" />
		<AlertDialog.Content class="dialog">
			<AlertDialog.Title>Delete this passkey?</AlertDialog.Title>
			<AlertDialog.Description>
				You won't be able to log in with it any more.
			</AlertDialog.Description>
			{#if deleteCredentialError}
				<p role="alert" class="error">{deleteCredentialError}</p>
			{/if}
			<div class="actions">
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
				<p role="alert" class="error">{revokeError}</p>
			{/if}
			<div class="actions">
				<AlertDialog.Cancel type="button">Cancel</AlertDialog.Cancel>
				<AlertDialog.Action type="button" onclick={confirmRevoke}>Revoke</AlertDialog.Action>
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

	section {
		margin-bottom: 3rem;
	}

	.section-header {
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

	.warning {
		color: var(--color-warning);
		font-weight: bold;
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
		margin-top: var(--space-3);
	}

	form input,
	:global(.dialog) textarea {
		width: 100%;
		box-sizing: border-box;
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
		margin-top: var(--space-4);
	}
</style>
