<script lang="ts">
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
import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
import * as Dialog from '$lib/components/ui/dialog/index.js';
import { Input } from '$lib/components/ui/input/index.js';
import { Label } from '$lib/components/ui/label/index.js';
import { Textarea } from '$lib/components/ui/textarea/index.js';
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
				<Dialog.Trigger class={buttonVariants({ variant: 'default' })}>
					Add passkey
				</Dialog.Trigger>
				<Dialog.Content>
					<Dialog.Header>
						<Dialog.Title>Add a passkey</Dialog.Title>
					</Dialog.Header>
					<form onsubmit={submitAdd}>
						<Label for="add-nickname">Nickname (optional)</Label>
						<Input id="add-nickname" class="mt-1 w-full" bind:value={addNickname} />

						{#if addError}
							<p role="alert" class="mt-3 text-error">{addError}</p>
						{/if}

						<Dialog.Footer>
							<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
								Cancel
							</Dialog.Close>
							<Button type="submit" disabled={addPending}>
								{addPending ? 'Waiting for your browser…' : 'Register'}
							</Button>
						</Dialog.Footer>
					</form>
				</Dialog.Content>
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
							<Button
								variant="outline"
								size="sm"
								onclick={() => openRename(credential.id, credential.nickname)}
							>
								Rename
							</Button>
							<Button
								variant="destructive"
								size="sm"
								onclick={() => openDeleteCredential(credential.id)}
							>
								Delete
							</Button>
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
				<Dialog.Trigger class={buttonVariants({ variant: 'default' })}>
					New token
				</Dialog.Trigger>
				<Dialog.Content>
					{#if createdToken}
						<Dialog.Header>
							<Dialog.Title>Token created</Dialog.Title>
						</Dialog.Header>
						<p role="alert" class="font-bold text-warning">
							This value is shown once. It will not be shown again - store it now.
						</p>
						<Textarea
							readonly
							rows={3}
							value={createdToken.value}
							aria-label="Token value"
							class="w-full"
						/>
						<Dialog.Footer>
							<Button onclick={closeCreateToken}>Done</Button>
						</Dialog.Footer>
					{:else}
						<Dialog.Header>
							<Dialog.Title>Create a token</Dialog.Title>
						</Dialog.Header>
						<form onsubmit={submitCreateToken}>
							<Label for="token-description">Description</Label>
							<Input
								id="token-description"
								class="mt-1 mb-3 w-full"
								bind:value={tokenDescription}
								required
							/>

							<Label for="token-ttl">Valid for (days)</Label>
							<Input
								id="token-ttl"
								class="mt-1 w-full"
								type="number"
								min="1"
								bind:value={tokenTTLDays}
								required
							/>

							{#if tokenError}
								<p role="alert" class="mt-3 text-error">{tokenError}</p>
							{/if}

							<Dialog.Footer>
								<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
									Cancel
								</Dialog.Close>
								<Button type="submit">Create</Button>
							</Dialog.Footer>
						</form>
					{/if}
				</Dialog.Content>
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
								<Button variant="destructive" size="sm" onclick={() => openRevoke(token.id)}>
									Revoke
								</Button>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</section>
</main>

<Dialog.Root bind:open={renameOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Rename passkey</Dialog.Title>
		</Dialog.Header>
		<form onsubmit={submitRename}>
			<Label for="rename-nickname">Nickname</Label>
			<Input id="rename-nickname" class="mt-1 w-full" bind:value={renameNickname} required />

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

<AlertDialog.Root bind:open={deleteCredentialOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete this passkey?</AlertDialog.Title>
			<AlertDialog.Description>
				You won't be able to log in with it any more.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if deleteCredentialError}
			<p role="alert" class="text-error">{deleteCredentialError}</p>
		{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDeleteCredential}>
				Delete
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={revokeOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Revoke this token?</AlertDialog.Title>
			<AlertDialog.Description>
				Anything still using it will stop being able to write.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if revokeError}
			<p role="alert" class="text-error">{revokeError}</p>
		{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmRevoke}>
				Revoke
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
