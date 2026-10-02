<script lang="ts">
import { goto, invalidate } from '$app/navigation';
import { page } from '$app/state';
import {
	ApiError,
	type ConsumerTokenWithValue,
	createConsumerToken,
	createToken,
	deleteCredential,
	purgeConsumerToken,
	purgeToken,
	renameCredential,
	revokeConsumerToken,
	revokeToken,
	rotateConsumerToken,
	rotateToken,
	type TokenWithValue,
} from '$lib/api';
import { registerPasskey } from '$lib/auth';
import ConsumerCombobox from '$lib/ConsumerCombobox.svelte';
import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
import * as Dialog from '$lib/components/ui/dialog/index.js';
import { Input } from '$lib/components/ui/input/index.js';
import { Label } from '$lib/components/ui/label/index.js';
import { Textarea } from '$lib/components/ui/textarea/index.js';
import { formatTimestamp, isTokenDead } from '$lib/datetime';
import TokenCard from '$lib/TokenCard.svelte';
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

// Consumer tokens

// The Consumers directory links here with ?consumer=<name> (design.md's
// "Filtering lives in settings/+page.svelte" decision - GET
// /consumer-tokens has no server-side per-consumer filter, so +page.ts
// still fetches every token and this derives the filtered view instead).
const consumerFilter = $derived(page.url.searchParams.get('consumer') ?? '');

const filteredConsumerTokens = $derived(
	consumerFilter
		? data.consumerTokens.filter((token) => token.consumer === consumerFilter)
		: data.consumerTokens,
);

function clearConsumerFilter() {
	const url = new URL(page.url);
	url.searchParams.delete('consumer');
	void goto(url, { replaceState: true, noScroll: true, keepFocus: true });
}

$effect(() => {
	if (consumerFilter) {
		document.getElementById('consumer-tokens')?.scrollIntoView();
	}
});

let createConsumerTokenOpen = $state(false);
let consumerTokenConsumer: string[] = $state([]);
let consumerTokenDescription = $state('');
let consumerTokenTTLDays = $state(90);
let consumerTokenError = $state('');
let createdConsumerToken: ConsumerTokenWithValue | null = $state(null);

function resetConsumerTokenForm() {
	consumerTokenConsumer = [];
	consumerTokenDescription = '';
	consumerTokenTTLDays = 90;
	consumerTokenError = '';
	createdConsumerToken = null;
}

async function submitCreateConsumerToken(event: SubmitEvent) {
	event.preventDefault();
	consumerTokenError = '';

	try {
		createdConsumerToken = await createConsumerToken(
			consumerTokenConsumer[0],
			consumerTokenDescription,
			consumerTokenTTLDays * 24 * 60 * 60,
		);
		await invalidate('app:settings');
	} catch (err) {
		consumerTokenError = apiErrorMessage(err, 'Failed to create the token.');
	}
}

function closeCreateConsumerToken() {
	createConsumerTokenOpen = false;
	resetConsumerTokenForm();
}

let revokeConsumerTokenOpen = $state(false);
let revokeConsumerTokenId = $state('');
let revokeConsumerTokenError = $state('');

function openRevokeConsumerToken(id: string) {
	revokeConsumerTokenId = id;
	revokeConsumerTokenError = '';
	revokeConsumerTokenOpen = true;
}

async function confirmRevokeConsumerToken() {
	revokeConsumerTokenError = '';

	try {
		await revokeConsumerToken(revokeConsumerTokenId);
		revokeConsumerTokenOpen = false;
		await invalidate('app:settings');
	} catch (err) {
		revokeConsumerTokenError = apiErrorMessage(
			err,
			'Failed to revoke the token.',
		);
	}
}

let rotateConsumerTokenOpen = $state(false);
let rotateConsumerTokenId = $state('');
let rotateConsumerTokenTTLDays = $state(90);
let rotateConsumerTokenError = $state('');
let rotatedConsumerToken: ConsumerTokenWithValue | null = $state(null);

function openRotateConsumerToken(id: string) {
	rotateConsumerTokenId = id;
	rotateConsumerTokenTTLDays = 90;
	rotateConsumerTokenError = '';
	rotatedConsumerToken = null;
	rotateConsumerTokenOpen = true;
}

async function submitRotateConsumerToken(event: SubmitEvent) {
	event.preventDefault();
	rotateConsumerTokenError = '';

	try {
		rotatedConsumerToken = await rotateConsumerToken(
			rotateConsumerTokenId,
			rotateConsumerTokenTTLDays * 24 * 60 * 60,
		);
		await invalidate('app:settings');
	} catch (err) {
		rotateConsumerTokenError = apiErrorMessage(
			err,
			'Failed to rotate the token.',
		);
	}
}

function closeRotateConsumerToken() {
	rotateConsumerTokenOpen = false;
	rotateConsumerTokenId = '';
	rotateConsumerTokenTTLDays = 90;
	rotateConsumerTokenError = '';
	rotatedConsumerToken = null;
}

// Purge (alrayyes/hush-hush#441) - a hard-delete restricted to a token
// that's already dead (revoked or past its expiry, per isTokenDead).

let purgeOpen = $state(false);
let purgeId = $state('');
let purgeError = $state('');

function openPurge(id: string) {
	purgeId = id;
	purgeError = '';
	purgeOpen = true;
}

async function confirmPurge() {
	purgeError = '';

	try {
		await purgeToken(purgeId);
		purgeOpen = false;
		await invalidate('app:settings');
	} catch (err) {
		purgeError = apiErrorMessage(err, 'Failed to delete the token.');
	}
}

let purgeConsumerTokenOpen = $state(false);
let purgeConsumerTokenId = $state('');
let purgeConsumerTokenError = $state('');

function openPurgeConsumerToken(id: string) {
	purgeConsumerTokenId = id;
	purgeConsumerTokenError = '';
	purgeConsumerTokenOpen = true;
}

async function confirmPurgeConsumerToken() {
	purgeConsumerTokenError = '';

	try {
		await purgeConsumerToken(purgeConsumerTokenId);
		purgeConsumerTokenOpen = false;
		await invalidate('app:settings');
	} catch (err) {
		purgeConsumerTokenError = apiErrorMessage(
			err,
			'Failed to delete the token.',
		);
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

		<ul aria-label="Bearer token list" class="m-0 list-none space-y-3 p-0 md:hidden">
			{#each data.tokens as token (token.id)}
				<TokenCard
					{token}
					subtitle={token.owner ?? 'cli'}
					onrotate={() => openRotate(token.id)}
					onrevoke={() => openRevoke(token.id)}
					onpurge={() => openPurge(token.id)}
				/>
			{/each}
		</ul>

		<table class="responsive-table hidden md:table">
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
						<td data-label="Status">
							{token.revoked ? 'Revoked' : isTokenDead(token) ? 'Expired' : 'Active'}
						</td>
						<td data-label="Actions" class="row-actions gap-3">
							{#if isTokenDead(token)}
								<Button variant="destructive" size="sm" onclick={() => openPurge(token.id)}>
									Delete permanently
								</Button>
							{:else}
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

	<section class="mb-12">
		<header class="flex flex-wrap items-center justify-between gap-2">
			<h2 id="consumer-tokens">Consumer tokens</h2>
			<Dialog.Root
				bind:open={createConsumerTokenOpen}
				onOpenChange={(open) => {
					if (!open) resetConsumerTokenForm();
				}}
			>
				<Dialog.Trigger class={buttonVariants({ variant: 'default' })}>
					New consumer token
				</Dialog.Trigger>
				<Dialog.Content>
					{#if createdConsumerToken}
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
								value={createdConsumerToken.value}
								aria-label="Token value"
								class="w-full"
							/>
						</div>
						<Dialog.Footer>
							<Button onclick={closeCreateConsumerToken}>Done</Button>
						</Dialog.Footer>
					{:else}
						<Dialog.Header>
							<Dialog.Title>Create a consumer token</Dialog.Title>
						</Dialog.Header>
						<form onsubmit={submitCreateConsumerToken}>
							<div class="space-y-4">
								<div class="space-y-1">
									<Label for="consumer-token-consumer">Consumer</Label>
									<ConsumerCombobox
										id="consumer-token-consumer"
										bind:value={consumerTokenConsumer}
										max={1}
										showKeyStatus={false}
									/>
								</div>

								<div class="space-y-1">
									<Label for="consumer-token-description">Description</Label>
									<Input
										id="consumer-token-description"
										class="w-full"
										bind:value={consumerTokenDescription}
										required
									/>
								</div>

								<div class="space-y-1">
									<Label for="consumer-token-ttl">Valid for (days)</Label>
									<Input
										id="consumer-token-ttl"
										class="w-full"
										type="number"
										min="1"
										bind:value={consumerTokenTTLDays}
										required
									/>
								</div>

								{#if consumerTokenError}
									<p role="alert" class="text-error">{consumerTokenError}</p>
								{/if}
							</div>

							<Dialog.Footer>
								<Dialog.Close class={buttonVariants({ variant: 'outline' })}>
									Cancel
								</Dialog.Close>
								<Button type="submit" disabled={consumerTokenConsumer.length === 0}>
									Create
								</Button>
							</Dialog.Footer>
						</form>
					{/if}
				</Dialog.Content>
			</Dialog.Root>
		</header>

		{#if consumerFilter}
			<ul class="mb-4 flex list-none flex-wrap gap-2 p-0">
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					consumer: {consumerFilter}
					<Button
						variant="ghost"
						size="icon-xs"
						class="ml-1 h-auto w-auto p-0"
						onclick={clearConsumerFilter}
						aria-label="Remove consumer filter"
					>
						&times;
					</Button>
				</li>
			</ul>
		{/if}

		<ul aria-label="Consumer token list" class="m-0 list-none space-y-3 p-0 md:hidden">
			{#each filteredConsumerTokens as token (token.id)}
				<TokenCard
					{token}
					subtitle={token.consumer}
					onrotate={() => openRotateConsumerToken(token.id)}
					onrevoke={() => openRevokeConsumerToken(token.id)}
					onpurge={() => openPurgeConsumerToken(token.id)}
				/>
			{/each}
		</ul>

		<table class="responsive-table hidden md:table">
			<thead>
				<tr>
					<th scope="col" class="px-4 py-3">Consumer</th>
					<th scope="col" class="px-4 py-3">Description</th>
					<th scope="col" class="px-4 py-3">Created</th>
					<th scope="col" class="px-4 py-3">Expires</th>
					<th scope="col" class="px-4 py-3">Last used</th>
					<th scope="col" class="px-4 py-3">Status</th>
					<th scope="col" class="px-4 py-3">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each filteredConsumerTokens as token (token.id)}
					<tr>
						<td data-label="Consumer">{token.consumer}</td>
						<td data-label="Description">{token.description}</td>
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
						<td data-label="Status">
							{token.revoked ? 'Revoked' : isTokenDead(token) ? 'Expired' : 'Active'}
						</td>
						<td data-label="Actions" class="row-actions gap-3">
							{#if isTokenDead(token)}
								<Button
									variant="destructive"
									size="sm"
									onclick={() => openPurgeConsumerToken(token.id)}
								>
									Delete permanently
								</Button>
							{:else}
								<Button
									variant="outline"
									size="sm"
									onclick={() => openRotateConsumerToken(token.id)}
								>
									Rotate
								</Button>
								<Button
									variant="destructive"
									size="sm"
									onclick={() => openRevokeConsumerToken(token.id)}
								>
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

<AlertDialog.Root bind:open={revokeConsumerTokenOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Revoke this token?</AlertDialog.Title>
			<AlertDialog.Description>
				That consumer will immediately lose read access.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if revokeConsumerTokenError}
			<p role="alert" class="text-error">{revokeConsumerTokenError}</p>
		{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmRevokeConsumerToken}>
				Revoke
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<Dialog.Root
	bind:open={rotateConsumerTokenOpen}
	onOpenChange={(open) => {
		if (!open) closeRotateConsumerToken();
	}}
>
	<Dialog.Content>
		{#if rotatedConsumerToken}
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
					value={rotatedConsumerToken.value}
					aria-label="Token value"
					class="w-full"
				/>
			</div>
			<Dialog.Footer>
				<Button onclick={closeRotateConsumerToken}>Done</Button>
			</Dialog.Footer>
		{:else}
			<Dialog.Header>
				<Dialog.Title>Rotate this token?</Dialog.Title>
			</Dialog.Header>
			<form onsubmit={submitRotateConsumerToken}>
				<div class="space-y-4">
					<div class="space-y-1">
						<Label for="rotate-consumer-token-ttl">Valid for (days)</Label>
						<Input
							id="rotate-consumer-token-ttl"
							class="w-full"
							type="number"
							min="1"
							bind:value={rotateConsumerTokenTTLDays}
							required
						/>
					</div>

					{#if rotateConsumerTokenError}
						<p role="alert" class="text-error">{rotateConsumerTokenError}</p>
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

<AlertDialog.Root bind:open={purgeOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete this token permanently?</AlertDialog.Title>
			<AlertDialog.Description>
				This can't be undone. Audit-log entries referencing it will show as
				unresolvable afterward.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if purgeError}
			<p role="alert" class="text-error">{purgeError}</p>
		{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmPurge}>
				Delete permanently
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={purgeConsumerTokenOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete this token permanently?</AlertDialog.Title>
			<AlertDialog.Description>
				This can't be undone. Audit-log entries referencing it will show as
				unresolvable afterward.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if purgeConsumerTokenError}
			<p role="alert" class="text-error">{purgeConsumerTokenError}</p>
		{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action
				variant="destructive"
				onclick={confirmPurgeConsumerToken}
			>
				Delete permanently
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
