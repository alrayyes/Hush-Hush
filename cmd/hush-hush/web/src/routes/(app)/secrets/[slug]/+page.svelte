<script lang="ts">
import { onDestroy } from 'svelte';
import { actorLabel } from '#lib/attribution.js';
import { createCopier } from '#lib/clipboard.js';
import { Button } from '#lib/components/ui/button/index.js';
import { Input } from '#lib/components/ui/input/index.js';
import { Textarea } from '#lib/components/ui/textarea/index.js';
import {
	CONSUMER_LIST_FILTER_MIN,
	consumersHref,
	filterConsumers,
	truncateKey,
} from '#lib/consumers.js';
import { formatTimestamp } from '#lib/datetime.js';
import { page } from '$app/state';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

// Which copy button last succeeded; reset after COPIED_MS. A rejected
// write leaves it unset so there's never a false "Copied".
let copied = $state<'sealed' | 'command' | ''>('');
const copier = createCopier<'sealed' | 'command'>(
	(kind) => (copied = kind ?? ''),
);
onDestroy(copier.dispose);

// A variant can have a hundred consumers: past a handful the section gets
// a filter box and scrolls in its own box instead of lengthening the page.
let consumerQuery = $state('');
const visibleConsumers = $derived.by(() => {
	if (data.notFound || data.chooseVariant) return [];
	const names = new Set(
		filterConsumers(
			data.consumers.map((c) => c.name),
			consumerQuery,
		),
	);

	return data.consumers.filter((c) => names.has(c.name));
});

const byteSize = $derived(
	data.notFound || data.chooseVariant ? 0 : atob(data.value).length,
);

const command = $derived(
	[
		`HUSH_HUSH_SERVER=${page.url.origin} \\`,
		'HUSH_HUSH_CONSUMER_TOKEN=<token> \\',
		`hush-hush-cli get ${data.slug} --identity "AGE-SECRET-KEY-1..."`,
	].join('\n'),
);
</script>

<main class="mx-auto my-8 max-w-240 px-4">
	<a href="/" class="text-sm underline">← Secrets</a>
	<h1 class="font-mono text-lg break-words">{data.slug}</h1>

	{#if data.notFound}
		<p role="alert">Secret not found.</p>
	{:else if data.chooseVariant}
		<section class="my-6 flex flex-col gap-2" aria-labelledby="variants-heading">
			<h2 id="variants-heading">Variants</h2>
			<p>This name holds {data.variants.length} variants, each with its own value. Pick one.</p>
			<ul class="flex flex-col gap-2">
				{#each data.variants as variant (variant.id)}
					<li>
						<a
							href={`/secrets/${encodeURIComponent(data.slug)}?id=${encodeURIComponent(variant.id)}`}
							class="underline"
						>
							{variant.used_by?.length ? variant.used_by.join(', ') : 'No consumers'}
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{:else}
		<section class="my-6 flex flex-col gap-2" aria-labelledby="sealed-heading">
			<h2 id="sealed-heading">Sealed ciphertext</h2>
			<Textarea
				readonly
				rows={6}
				value={data.value}
				aria-label="Sealed ciphertext (base64)"
				class="font-mono text-xs"
			/>
			<div class="flex flex-wrap items-center gap-3">
				<Button
					variant="outline"
					class="min-h-11"
					aria-label="Copy sealed ciphertext"
					onclick={() => copier.copy('sealed', data.value)}
				>
					{copied === 'sealed' ? 'Copied' : 'Copy'}
				</Button>
				<span>{byteSize} bytes</span>
			</div>
			<p>
				The server only holds this ciphertext. It's decrypted locally, with your private key.
			</p>
		</section>

		<section class="my-6 flex flex-col gap-2" aria-labelledby="cli-heading">
			<h2 id="cli-heading">Fetch with the CLI</h2>
			<div role="group" aria-label="Fetch with the CLI" class="flex flex-col gap-2">
				<pre class="font-mono text-xs break-words whitespace-pre-wrap">{command}</pre>
				<div>
					<Button variant="outline" class="min-h-11" onclick={() => copier.copy('command', command)}>
						{copied === 'command' ? 'Copied' : 'Copy command'}
					</Button>
				</div>
			</div>
		</section>

		<section class="my-6 flex flex-col gap-2" aria-labelledby="consumers-heading">
			<h2 id="consumers-heading">Authorized consumers</h2>
			{#if data.consumers.length === 0}
				<p>No recorded consumers.</p>
			{:else}
				{#if data.consumers.length > CONSUMER_LIST_FILTER_MIN}
					<Input
						type="search"
						class="w-full"
						placeholder="Filter consumers"
						aria-label="Filter consumers"
						bind:value={consumerQuery}
					/>
					<p class="m-0 text-sm text-text-muted" role="status">
						{visibleConsumers.length} of {data.consumers.length} shown
					</p>
				{/if}
				<ul class="flex max-h-96 flex-col gap-2 overflow-y-auto">
					{#each visibleConsumers as consumer (consumer.name)}
						<li class="flex flex-wrap items-center gap-2">
							<a href={consumersHref(1, consumer.name)} class="underline">{consumer.name}</a>
							{#if consumer.publicKey}
								<code class="font-mono text-xs">{truncateKey(consumer.publicKey)}</code>
							{:else}
								<span>not registered</span>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<section class="my-6 flex flex-col gap-2" aria-labelledby="activity-heading">
			<h2 id="activity-heading">Recent activity</h2>
			{#if data.events.length === 0}
				<p>No activity recorded.</p>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each data.events as entry (entry.id)}
						<li class="flex flex-wrap items-center gap-2">
							<span>{entry.action}</span>
							<time datetime={entry.timestamp} title={entry.timestamp}>
								{formatTimestamp(entry.timestamp)}
							</time>
							<span>by {actorLabel(entry)}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	{/if}
</main>
