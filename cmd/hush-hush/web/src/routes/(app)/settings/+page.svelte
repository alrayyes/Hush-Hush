<script lang="ts">
import { invalidate } from '$app/navigation';
import {
	ApiError,
	createToken,
	deleteCredential,
	renameCredential,
	revokeToken,
	rotateToken,
	type TokenWithValue,
} from '$lib/api';
import { registerPasskey } from '$lib/auth';
import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
import * as Dialog from '$lib/components/ui/dialog/index.js';
import { Input } from '$lib/components/ui/input/index.js';
import { Label } from '$lib/components/ui/label/index.js';
import { Textarea } from '$lib/components/ui/textarea/index.js';
import { formatTimestamp } from '$lib/datetime';
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

let rotateOpen = $state(false);
let rotateId = $state('');
let rotateTTLDays = $state(90);
let rotateError = $state('');
let rotatedToken: TokenWithValue | null = $state(null);

function openRotate(id: string) {
	rotateId = id;
	rotateTTLDays = 90;
	rotateError = '';
	rotatedToken = null;
	rotateOpen = true;
}

async function submitRotate(event: SubmitEvent) {
	event.preventDefault();
	rotateError = '';

	try {
		rotatedToken = await rotateToken(rotateId, rotateTTLDays * 24 * 60 * 60);
		await invalidate('app:settings');
	} catch (err) {
		rotateError = apiErrorMessage(err, 'Failed to rotate the token.');
	}
}

function closeRotate() {
	rotateOpen = false;
	rotateId = '';
	rotateTTLDays = 90;
	rotateError = '';
	rotatedToken = null;
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
						<div class="space-y-4">
							<div class="space-y-1">
								<Label for="add-nickname">Nickname (optional)</Label>
								<Input id="add-nickname" class="w-full" bind:value={addNickname} />
							</div>

							{#if addError}
								<p role="alert" class="text-error">{addError}</p>
							{/if}
						</div>

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
					<th scope="col" class="px-4 py-3">Nickname</th>
					<th scope="col" class="px-4 py-3">Added</th>
					<th scope="col" class="px-4 py-3">Last used</th>
					<th scope="col" class="px-4 py-3">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each data.credentials as credential (credential.id)}
					<tr>
						<td data-label="Nickname">{credential.nickname ?? ''}</td>
						<td data-label="Added">
							<time datetime={credential.created_at} title={credential.created_at}>
								{formatTimestamp(credential.created_at)}
							</time>
						</td>
						<td data-label="Last used">
							{#if credential.last_used_at}
								<time datetime={credential.last_used_at} title={credential.last_used_at}>
									{formatTimestamp(credential.last_used_at)}
								</time>
							{:else}
								never
							{/if}
						</td>
						<td data-label="Actions" class="row-actions gap-3">
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
						<div class="space-y-4">
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
						</div>
						<Dialog.Footer>
							<Button onclick={closeCreateToken}>Done</Button>
						</Dialog.Footer>
					{:else}
						<Dialog.Header>
							<Dialog.Title>Create a token</Dialog.Title>
						</Dialog.Header>
						<form onsubmit={submitCreateToken}>
							<div class="space-y-4">
								<div class="space-y-1">
									<Label for="token-description">Description</Label>
									<Input
										id="token-description"
										class="w-full"
										bind:value={tokenDescription}
										required
									/>
								</div>

								<div class="space-y-1">
									<Label for="token-ttl">Valid for (days)</Label>
									<Input
										id="token-ttl"
										class="w-full"
										type="number"
										min="1"
										bind:value={tokenTTLDays}
										required
									/>
								</div>

								{#if tokenError}
									<p role="alert" class="text-error">{tokenError}</p>
								{/if}
							</div>

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
					<th scope="col" class="px-4 py-3">Description</th>
					<th scope="col" class="px-4 py-3">Owner</th>
					<th scope="col" class="px-4 py-3">Created</th>
					<th scope="col" class="px-4 py-3">Expires</th>
					<th scope="col" class="px-4 py-3">Last used</th>
					<th scope="col" class="px-4 py-3">Status</th>
					<th scope="col" class="px-4 py-3">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each data.tokens as token (token.id)}
					<tr>
						<td data-label="Description">{token.description}</td>
						<td data-label="Owner">{token.owner ?? 'cli'}</td>
						<td data-label="Created">
							<time datetime={token.created_at} title={token.created_at}>
								{formatTimestamp(token.created_at)}
							</time>
						</td>
						<td data-label="Expires">
							<time datetime={token.expires_at} title={token.expires_at}>
								{formatTimestamp(token.expires_at)}
							</time>
						</td>
						<td data-label="Last used">
							{#if token.last_used_at}
								<time datetime={token.last_used_at} title={token.last_used_at}>
									{formatTimestamp(token.last_used_at)}
								</time>
							{:else}
								never
							{/if}
						</td>
						<td data-label="Status">{token.revoked ? 'Revoked' : 'Active'}</td>
						<td data-label="Actions" class="row-actions gap-3">
							{#if !token.revoked}
								<Button variant="outline" size="sm" onclick={() => openRotate(token.id)}>
									Rotate
								</Button>
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
			<div class="space-y-4">
				<div class="space-y-1">
					<Label for="rename-nickname">Nickname</Label>
					<Input id="rename-nickname" class="w-full" bind:value={renameNickname} required />
				</div>

				{#if renameError}
					<p role="alert" class="text-error">{renameError}</p>
				{/if}
			</div>

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

<Dialog.Root
	bind:open={rotateOpen}
	onOpenChange={(open) => {
		if (!open) closeRotate();
	}}
>
	<Dialog.Content>
		{#if rotatedToken}
			<Dialog.Header>
				<Dialog.Title>Token rotated</Dialog.Title>
			</Dialog.Header>
			<div class="space-y-4">
				<p role="alert" class="font-bold text-warning">
					This value is shown once. It will not be shown again - store it now.
				</p>
				<Textarea
					readonly
					rows={3}
					value={rotatedToken.value}
					aria-label="Token value"
					class="w-full"
				/>
			</div>
			<Dialog.Footer>
				<Button onclick={closeRotate}>Done</Button>
			</Dialog.Footer>
		{:else}
			<Dialog.Header>
				<Dialog.Title>Rotate this token?</Dialog.Title>
			</Dialog.Header>
			<form onsubmit={submitRotate}>
				<div class="space-y-4">
					<div class="space-y-1">
						<Label for="rotate-ttl">Valid for (days)</Label>
						<Input
							id="rotate-ttl"
							class="w-full"
							type="number"
							min="1"
							bind:value={rotateTTLDays}
							required
						/>
					</div>

					{#if rotateError}
						<p role="alert" class="text-error">{rotateError}</p>
					{/if}
				</div>

				<Dialog.Footer>
					<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
						Cancel
					</Dialog.Close>
					<Button type="submit">Rotate</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
