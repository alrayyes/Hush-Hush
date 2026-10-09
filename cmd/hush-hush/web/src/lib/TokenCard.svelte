<script lang="ts">
import type { TokenAction, TokenStatus } from '#lib/api.js';
import { Button } from '#lib/components/ui/button/index.js';
import { formatTimestamp } from '#lib/datetime.js';
import { tokenRemaining, tokenStatusLabel } from '#lib/tokens.js';

// One token as a card, for the below-md list on the Settings page
// (alrayyes/hush-hush#484). Bearer and consumer tokens share the shape
// apart from the line under the description, so `subtitle` carries the
// owner or the consumer name and the three handlers carry the dialogs.
let {
	token,
	subtitle,
	showId = false,
	onrotate,
	onrevoke,
	onpurge,
}: {
	token: {
		id?: string;
		description: string;
		created_at: string;
		expires_at: string;
		revoked: boolean;
		status?: TokenStatus;
		allowed_actions?: TokenAction[];
		last_used_at?: string;
	};
	subtitle: string;
	// Consumer tokens show their id: two for one consumer and description
	// differ in nothing else a reader can see.
	showId?: boolean;
	onrotate: () => void;
	onrevoke: () => void;
	onpurge: () => void;
} = $props();

const statusLabel = $derived(tokenStatusLabel(token.status));
const remaining = $derived(tokenRemaining(token));
const badgeClass = $derived(
	token.status === 'revoked' || token.status === 'expired'
		? 'border-error text-error'
		: remaining.endsWith('m left') || remaining.endsWith('h left')
			? 'border-warning text-warning'
			: 'border-border text-text-muted',
);
</script>

<li class="space-y-2 rounded-lg border border-border bg-background p-4">
	<div class="flex items-start justify-between gap-2">
		<span class="min-w-0 break-words font-semibold">{token.description}</span>
		<span
			data-testid="ttl-badge"
			class="shrink-0 rounded-full border px-2 py-0.5 text-xs font-semibold {badgeClass}"
		>
			{remaining}
		</span>
	</div>
	<p class="m-0 text-sm text-text-muted">{subtitle}</p>
	{#if showId && token.id}
		<p class="m-0 text-xs text-text-muted">
			ID <code data-testid="token-id" class="font-mono select-all">{token.id}</code>
		</p>
	{/if}
	<p class="m-0 text-sm">{statusLabel}</p>
	<dl class="m-0 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
		<dt class="text-text-muted">Created</dt>
		<dd class="m-0">
			<time datetime={token.created_at} title={token.created_at}>
				{formatTimestamp(token.created_at)}
			</time>
		</dd>
		<dt class="text-text-muted">Expires</dt>
		<dd class="m-0">
			<time datetime={token.expires_at} title={token.expires_at}>
				{formatTimestamp(token.expires_at)}
			</time>
		</dd>
		<dt class="text-text-muted">Last used</dt>
		<dd class="m-0">
			{#if token.last_used_at}
				<time datetime={token.last_used_at} title={token.last_used_at}>
					{formatTimestamp(token.last_used_at)}
				</time>
			{:else}
				never
			{/if}
		</dd>
	</dl>
	<div class="flex flex-wrap gap-3">
		{#if token.allowed_actions?.includes('rotate')}
			<Button variant="outline" class="min-h-11 min-w-11" onclick={onrotate}>Rotate</Button>
		{/if}
		{#if token.allowed_actions?.includes('revoke')}
			<Button variant="destructive" class="min-h-11 min-w-11" onclick={onrevoke}>Revoke</Button>
		{/if}
		{#if token.allowed_actions?.includes('purge')}
			<Button variant="destructive" class="min-h-11 min-w-11" onclick={onpurge}>
				Delete permanently
			</Button>
		{/if}
	</div>
</li>
