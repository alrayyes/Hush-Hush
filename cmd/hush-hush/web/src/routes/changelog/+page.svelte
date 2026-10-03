<script lang="ts">
import { onDestroy } from 'svelte';
import { imageTag, parseChangelog, releaseUrl } from '$lib/changelog';
import { createCopier } from '$lib/clipboard';
import { Button } from '$lib/components/ui/button/index.js';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

const releases = $derived(data.changelog ? parseChangelog(data.changelog) : []);

// Colour never carries the category alone: the badge always shows the word.
function badgeClass(badge: string): string {
	if (badge === 'FEATURE') return 'border-accent text-accent';
	if (badge === 'SECURITY' || badge === 'BREAKING') {
		return 'border-error text-error';
	}
	return 'border-border text-text-muted';
}

// Which release's copy button is showing "Copied", and the timer that
// reverts it. One slot, so a newer copy replaces an older one.
let copiedVersion: string | null = $state(null);
const copier = createCopier<string>((version) => (copiedVersion = version));
onDestroy(copier.dispose);
</script>

<svelte:head>
	<title>Changelog - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-240 px-4">
	<h1>Changelog</h1>
	{#if releases.length > 0}
		<div class="space-y-4">
			{#each releases as release (release.version)}
				<article
					aria-labelledby="release-{release.version}"
					class="space-y-3 rounded-lg border border-border bg-background p-4"
				>
					<div class="flex flex-wrap items-baseline justify-between gap-2">
						<h2 id="release-{release.version}" class="m-0 font-mono text-lg font-semibold">
							v{release.version}
						</h2>
						<time datetime={release.date} class="text-sm text-text-muted">{release.date}</time>
					</div>
					<ul class="m-0 list-none space-y-2 p-0">
						{#each release.entries as entry, i (i)}
							<li class="flex flex-wrap items-center gap-x-2 gap-y-1 break-words text-sm">
								<span
									data-testid="entry-badge"
									class="rounded-full border px-2 py-0.5 text-xs font-semibold {badgeClass(entry.badge)}"
									>{entry.badge}</span
								>
								{#if entry.scope}
									<code class="font-mono text-xs break-all">{entry.scope}</code>
								{/if}
								<span class="min-w-0 break-words">{entry.text}</span>
								{#each entry.refs as ref (ref.url)}
									<a href={ref.url} rel="noreferrer" class="text-xs">{ref.label}</a>
								{/each}
							</li>
						{/each}
					</ul>
					<div class="flex flex-wrap items-center gap-3">
						<code class="font-mono text-xs break-all">{imageTag(release.version)}</code>
						<Button
							variant="outline"
							class="min-h-11 min-w-11"
							aria-label={copiedVersion === release.version
								? `Copied image tag for ${release.version}`
								: `Copy image tag for ${release.version}`}
							onclick={() => copier.copy(release.version, imageTag(release.version))}
						>
							{copiedVersion === release.version ? 'Copied' : 'Copy'}
						</Button>
						<Button
							variant="outline"
							class="min-h-11 min-w-11"
							href={releaseUrl(release.version)}
							rel="noopener noreferrer"
							aria-label="Release page for {release.version}"
						>
							Release page
						</Button>
					</div>
				</article>
			{/each}
		</div>
	{:else}
		<p>No changelog available.</p>
	{/if}
</main>
