<script lang="ts">
import { page } from '$app/state';
import { actorLabel } from '$lib/attribution';
import { Button } from '$lib/components/ui/button/index.js';
import { Textarea } from '$lib/components/ui/textarea/index.js';
import { consumersHref, truncateKey } from '$lib/consumers';
import { formatTimestamp } from '$lib/datetime';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

const COPIED_MS = 1500;

// Which copy button last succeeded; reset after COPIED_MS. A rejected
// write leaves it unset so there's never a false "Copied".
let copied = $state<'sealed' | 'command' | ''>('');
let copyTimer: ReturnType<typeof setTimeout> | undefined;

async function copy(kind: 'sealed' | 'command', text: string) {
	try {
		await navigator.clipboard.writeText(text);
	} catch {
		return;
	}

	copied = kind;
	clearTimeout(copyTimer);
	copyTimer = setTimeout(() => {
		copied = '';
	}, COPIED_MS);
}

const byteSize = $derived(data.notFound ? 0 : atob(data.value).length);

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
					onclick={() => copy('sealed', data.value)}
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
					<Button variant="outline" class="min-h-11" onclick={() => copy('command', command)}>
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
				<ul class="flex flex-col gap-2">
					{#each data.consumers as consumer (consumer.name)}
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
