<script lang="ts">
import CheckIcon from '@lucide/svelte/icons/check';
import CopyIcon from '@lucide/svelte/icons/copy';
import { onDestroy } from 'svelte';
import { createCopier } from '#lib/clipboard.js';
import { Button } from '#lib/components/ui/button/index.js';
import { errorCopy } from '#lib/error-copy.js';
import { page } from '$app/state';

// Renders inside the root layout, so the top bar, theme and footer are
// already around it. A failure in the root layout itself never gets here;
// src/error.html covers that.
const copy = $derived(errorCopy(page.status));
const authenticated = $derived(page.data.authenticated === true);

const CLI_COMMAND = 'hush-hush-cli get <slug> --identity "AGE-SECRET-KEY-1..."';

let copied = $state(false);
const copier = createCopier<boolean>((value) => (copied = value !== null));
onDestroy(copier.dispose);
</script>

<svelte:head>
	<title>{copy.title} - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-240 px-4">
	<section class="rounded-lg border border-border bg-surface p-6">
		<div class="flex flex-wrap items-center gap-3">
			<span
				class="rounded border border-border px-2 py-0.5 font-mono text-sm font-bold"
				>{page.status}</span
			>
			<h1 class="m-0">{copy.title}</h1>
		</div>
		<p class="mt-3 max-w-prose text-text-muted">{copy.message}</p>
	</section>

	<h2 class="mt-8 text-lg">Where to next</h2>
	<ul class="m-0 mt-3 grid list-none gap-3 p-0 md:grid-cols-3">
		<li class="space-y-2 rounded-lg border border-border bg-background p-4">
			<h3 class="m-0 text-base">
				{authenticated ? 'Back to the secrets list' : 'Sign in'}
			</h3>
			<p class="m-0 text-sm text-text-muted">
				{authenticated
					? 'Pick up where you left off.'
					: 'Sign in with your passkey to get to your secrets.'}
			</p>
			<Button href="/" variant="outline" class="mt-2 min-h-11">
				{authenticated ? 'Open secrets' : 'Go to sign in'}
			</Button>
		</li>
		{#if authenticated}
			<li class="space-y-2 rounded-lg border border-border bg-background p-4">
				<h3 class="m-0 text-base">Check the audit log</h3>
				<p class="m-0 text-sm text-text-muted">
					See what was created, read or removed, and when.
				</p>
				<Button href="/audit-log" variant="outline" class="mt-2 min-h-11">
					Open audit log
				</Button>
			</li>
		{/if}
		<li class="space-y-2 rounded-lg border border-border bg-background p-4">
			<h3 class="m-0 text-base">Fetch from the command line</h3>
			<p class="m-0 text-sm text-text-muted">
				The CLI reads a secret with your own key, with no browser involved.
			</p>
			<div class="flex items-center justify-between gap-2">
				<code class="min-w-0 break-all font-mono text-xs">{CLI_COMMAND}</code>
				<Button
					variant="outline"
					class="min-h-11 min-w-11"
					aria-label={copied ? 'Copied CLI command' : 'Copy CLI command'}
					onclick={() => copier.copy(true, CLI_COMMAND)}
				>
					{#if copied}
						<CheckIcon aria-hidden="true" />
					{:else}
						<CopyIcon aria-hidden="true" />
					{/if}
				</Button>
			</div>
		</li>
	</ul>
</main>
