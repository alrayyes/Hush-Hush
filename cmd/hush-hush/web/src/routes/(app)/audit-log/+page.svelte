<script lang="ts">
import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
import CheckIcon from '@lucide/svelte/icons/check';
import CopyIcon from '@lucide/svelte/icons/copy';
import { onDestroy } from 'svelte';
import {
	ApiError,
	type AuditLogEntry,
	type AuditLogQuery,
	queryAuditLog,
} from '#lib/api.js';
import { auditActorLabel, toCSV, toJSON } from '#lib/audit-export.js';
import {
	AUDIT_PAGE_SIZE,
	type AuditOrder,
	cursorParams,
	flipOrder,
	latestOnly,
	type PageMove,
	type PageStep,
	pagerState,
	turnPage,
} from '#lib/audit-paging.js';
import CursorPager from '#lib/CursorPager.svelte';
import { createCopier } from '#lib/clipboard.js';
import { Button } from '#lib/components/ui/button/index.js';
import { Input } from '#lib/components/ui/input/index.js';
import { Label } from '#lib/components/ui/label/index.js';
import * as Select from '#lib/components/ui/select/index.js';
import { formatTimestamp } from '#lib/datetime.js';
import { page } from '$app/state';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

// Seeded once from the load function's own unfiltered first page, then
// owned entirely by fetchPage below - this page never calls
// invalidate(), so there's nothing for `data` itself to update again
// with; every filter/pagination change refetches directly instead.
// svelte-check's state_referenced_locally warning is exactly this
// intentional one-time seed, not a sync bug.
let entries: AuditLogEntry[] = $state(data.entries);
let hasMore = $state(data.hasMore);
let loading = $state(false);
let loadError = $state('');

// Applied filters - object/actor/caller are select boxes populated from
// data.filterOptions (alrayyes/hush-hush#323), from/to are datetime-local
// strings converted to RFC 3339 on request. "" means no filter for all
// three selects, matching the free-text fields' own empty-string unset
// state before this change. Removing a chip (or changing a field)
// refetches immediately, no "Apply" step (design.md's "Audit log UI"
// decision).
let objectFilter = $state('');
let actorFilter = $state('');
let callerFilter = $state('');
let fromFilter = $state('');
let toFilter = $state('');

// The filters and order the table on screen was fetched with. A change whose
// fetch fails puts the controls back to these, so the controls, the table and
// the curl command always describe the same query (alrayyes/Hush-Hush#921).
let applied = {
	object: '',
	actor: '',
	caller: '',
	from: '',
	to: '',
	order: 'desc' as AuditOrder,
};

// Read when a request starts, not when it ends: a From or To still being typed
// when a Newer press resolves isn't part of what that request fetched.
function snapshotControls() {
	return {
		object: objectFilter,
		actor: actorFilter,
		caller: callerFilter,
		from: fromFilter,
		to: toFilter,
		order,
	};
}

function restoreControls() {
	objectFilter = applied.object;
	actorFilter = applied.actor;
	callerFilter = applied.caller;
	fromFilter = applied.from;
	toFilter = applied.to;
	order = applied.order;
}

// cursorStack holds the `after` value that produced each earlier page,
// so "Previous" can pop back to it - the API itself only ever pages
// forward (design.md's "Audit log UI" decision: id-based cursor, no
// offset), so "previous" is this page's own bookkeeping, not a second
// server capability.
let cursorStack: (number | undefined)[] = $state([]);
let currentAfter: number | undefined = $state(undefined);

// Newest first, like the load function's own first page: an operator reads
// an audit log for what just happened (alrayyes/hush-hush#753). The cursor
// that pages onward depends on it, see cursorParams.
let order: AuditOrder = $state('desc');

function currentQuery(after: number | undefined): AuditLogQuery {
	return {
		object_id: objectFilter || undefined,
		actor: actorFilter || undefined,
		caller: callerFilter || undefined,
		from: fromFilter ? new Date(fromFilter).toISOString() : undefined,
		to: toFilter ? new Date(toFilter).toISOString() : undefined,
		order,
		limit: AUDIT_PAGE_SIZE,
		...cursorParams(order, after),
	};
}

// Turns to another page. The cursor stack, the entries and the flag only change
// together, after the load succeeds (alrayyes/Hush-Hush#906): a failed load
// shows the error and leaves Newer and Older where they were.
const startRequest = latestOnly();

async function go(move: PageMove) {
	const isCurrent = startRequest();
	const requested = snapshotControls();
	loading = true;
	loadError = '';

	try {
		const next = await turnPage(
			{ entries, hasMore, after: currentAfter, stack: cursorStack },
			move,
			// One row more than a page, to learn whether another page follows.
			(after) =>
				queryAuditLog({
					...currentQuery(after),
					limit: AUDIT_PAGE_SIZE + 1,
				}),
			AUDIT_PAGE_SIZE,
		);
		if (!isCurrent()) return;
		entries = next.entries;
		hasMore = next.hasMore;
		currentAfter = next.after;
		cursorStack = next.stack;
		applied = requested;
	} catch (err) {
		if (!isCurrent()) return;
		restoreControls();
		loadError =
			err instanceof ApiError ? err.message : 'Failed to load the audit log.';
	} finally {
		if (isCurrent()) loading = false;
	}
}

function resetToFirstPage() {
	void go('first');
}

function step(direction: PageStep) {
	void go(direction);
}

const pager = $derived(pagerState(order, cursorStack.length > 0, hasMore));

function toggleOrder() {
	order = flipOrder(order);
	resetToFirstPage();
}

function clearFilter(name: 'object' | 'actor' | 'caller' | 'from' | 'to') {
	if (name === 'object') objectFilter = '';
	if (name === 'actor') actorFilter = '';
	if (name === 'caller') callerFilter = '';
	if (name === 'from') fromFilter = '';
	if (name === 'to') toFilter = '';
	resetToFirstPage();
}

// actorLabel resolves a filter-options value back to its label, for the
// select's own closed-state display and for the applied-filter chip -
// the value sent to the server (a raw token id, or the "none" sentinel)
// isn't what a visitor should read back.
function actorLabel(value: string): string {
	return (
		data.filterOptions.actors.find((a) => a.value === value)?.label ?? value
	);
}

// The same query the table shows, as a copyable curl command. Built from
// the applied filters only (undefined values dropped). GET /audit-log is
// unauthenticated (api/openapi.yaml declares no security for it, inheriting
// the document's `security: []`), so the command carries no auth header.
const curlCommand = $derived.by(() => {
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(currentQuery(currentAfter))) {
		if (value !== undefined) params.set(key, String(value));
	}
	const qs = params.toString();
	return `curl -s "${page.url.origin}/audit-log${qs ? `?${qs}` : ''}" | jq`;
});

// Whether the curl copy button is showing its "Copied" state, and the
// timer that reverts it; a newer copy replaces an older timer.
let copied = $state(false);
const copier = createCopier<true>((copiedNow) => (copied = copiedNow === true));
onDestroy(copier.dispose);

const actionPillClass: Record<string, string> = {
	create: 'border-accent text-accent',
	read: 'border-border text-text-muted',
	update: 'border-warning text-warning',
	delete: 'border-error text-error',
};

function downloadBlob(content: string, mimeType: string, filename: string) {
	const blob = new Blob([content], { type: mimeType });
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	a.click();
	URL.revokeObjectURL(url);
}

function exportJSON() {
	downloadBlob(toJSON(entries), 'application/json', 'audit-log.json');
}

function exportCSV() {
	downloadBlob(toCSV(entries), 'text/csv', 'audit-log.csv');
}
</script>

<svelte:head>
	<title>Audit log - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-280 px-4">
	<h1>Audit log</h1>

	<form
		class="mb-4 grid grid-cols-[repeat(auto-fit,minmax(10rem,1fr))] gap-x-4 gap-y-3"
		onsubmit={(event) => {
			event.preventDefault();
			resetToFirstPage();
		}}
	>
		<div class="flex flex-col gap-1">
			<Label id="filter-object-label">Object id</Label>
			<Select.Root
				type="single"
				bind:value={objectFilter}
				onValueChange={resetToFirstPage}
			>
				<Select.Trigger aria-labelledby="filter-object-label" class="w-full">
					<Select.Value placeholder="Any object" />
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="" label="Any object" />
					{#each data.filterOptions.object_ids as objectID (objectID)}
						<Select.Item value={objectID} label={objectID} />
					{/each}
				</Select.Content>
			</Select.Root>
		</div>

		<div class="flex flex-col gap-1">
			<Label id="filter-actor-label">Actor</Label>
			<Select.Root
				type="single"
				bind:value={actorFilter}
				onValueChange={resetToFirstPage}
			>
				<Select.Trigger aria-labelledby="filter-actor-label" class="w-full">
					<Select.Value placeholder="Any actor" />
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="" label="Any actor" />
					{#each data.filterOptions.actors as actor (actor.value)}
						<Select.Item value={actor.value} label={actor.label} />
					{/each}
				</Select.Content>
			</Select.Root>
		</div>

		<div class="flex flex-col gap-1">
			<Label id="filter-caller-label">Caller</Label>
			<Select.Root
				type="single"
				bind:value={callerFilter}
				onValueChange={resetToFirstPage}
			>
				<Select.Trigger aria-labelledby="filter-caller-label" class="w-full">
					<Select.Value placeholder="Any caller" />
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="" label="Any caller" />
					{#each data.filterOptions.callers as caller (caller)}
						<Select.Item value={caller} label={caller} />
					{/each}
				</Select.Content>
			</Select.Root>
		</div>

		<div class="flex flex-col gap-1">
			<label class="text-sm" for="filter-from">From</label>
			<Input
				id="filter-from"
				class="w-full"
				type="datetime-local"
				bind:value={fromFilter}
				onchange={resetToFirstPage}
			/>
		</div>

		<div class="flex flex-col gap-1">
			<label class="text-sm" for="filter-to">To</label>
			<Input
				id="filter-to"
				class="w-full"
				type="datetime-local"
				bind:value={toFilter}
				onchange={resetToFirstPage}
			/>
		</div>
	</form>

	{#if objectFilter || actorFilter || callerFilter || fromFilter || toFilter}
		<ul class="mb-4 flex list-none flex-wrap gap-2 p-0">
			{#if objectFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					object: {objectFilter}
					<Button
						variant="ghost"
						size="icon-xs"
						class="ml-1 h-auto w-auto p-0"
						onclick={() => clearFilter('object')}
						aria-label="Remove object filter"
					>
						&times;
					</Button>
				</li>
			{/if}
			{#if actorFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					actor: {actorLabel(actorFilter)}
					<Button
						variant="ghost"
						size="icon-xs"
						class="ml-1 h-auto w-auto p-0"
						onclick={() => clearFilter('actor')}
						aria-label="Remove actor filter"
					>
						&times;
					</Button>
				</li>
			{/if}
			{#if callerFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					caller: {callerFilter}
					<Button
						variant="ghost"
						size="icon-xs"
						class="ml-1 h-auto w-auto p-0"
						onclick={() => clearFilter('caller')}
						aria-label="Remove caller filter"
					>
						&times;
					</Button>
				</li>
			{/if}
			{#if fromFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					from: {fromFilter}
					<Button
						variant="ghost"
						size="icon-xs"
						class="ml-1 h-auto w-auto p-0"
						onclick={() => clearFilter('from')}
						aria-label="Remove from filter"
					>
						&times;
					</Button>
				</li>
			{/if}
			{#if toFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					to: {toFilter}
					<Button
						variant="ghost"
						size="icon-xs"
						class="ml-1 h-auto w-auto p-0"
						onclick={() => clearFilter('to')}
						aria-label="Remove to filter"
					>
						&times;
					</Button>
				</li>
			{/if}
		</ul>
	{/if}

	<div class="mb-4 flex gap-2">
		<Button variant="outline" class="min-h-11 min-w-11" onclick={exportCSV}>
			Export CSV
		</Button>
		<Button variant="outline" class="min-h-11 min-w-11" onclick={exportJSON}>
			Export JSON
		</Button>
	</div>

	{#if loadError}
		<p role="alert" class="text-error">{loadError}</p>
	{/if}

	<ul aria-label="Audit events" class="m-0 list-none space-y-3 p-0 md:hidden">
		{#each entries as entry (entry.id)}
			<li class="space-y-2 rounded-lg border border-border bg-background p-4">
				<div class="flex items-start justify-between gap-2">
					<span class="min-w-0 font-mono font-semibold break-words">{entry.object_id}</span>
					<span
						data-testid="action-pill"
						class="rounded-full border px-2 py-0.5 text-xs font-semibold uppercase {actionPillClass[
							entry.action
						] ?? 'border-border text-text-muted'}"
					>{entry.action}</span>
				</div>
				<dl class="m-0 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
					<dt class="text-text-muted">Actor</dt>
					<dd class="m-0 break-words">{auditActorLabel(entry)}</dd>
					{#if entry.caller}
						<dt class="text-text-muted">Caller</dt>
						<dd class="m-0 break-words">{entry.caller}</dd>
					{/if}
					<dt class="text-text-muted">IP</dt>
					<dd class="m-0 break-words">{entry.ip}</dd>
					<dt class="text-text-muted">Time</dt>
					<dd class="m-0">
						<time datetime={entry.timestamp} title={entry.timestamp}>
							{formatTimestamp(entry.timestamp)}
						</time>
					</dd>
				</dl>
			</li>
		{/each}
	</ul>

	<table class="responsive-table hidden md:table" aria-busy={loading}>
		<thead>
			<tr>
				<th scope="col" class="px-4 py-3">Object</th>
				<th scope="col" class="px-4 py-3">Action</th>
				<th scope="col" class="px-4 py-3">Actor</th>
				<th scope="col" class="px-4 py-3">Caller</th>
				<th scope="col" class="px-4 py-3">IP</th>
				<th
					scope="col"
					class="px-4 py-3"
					aria-sort={order === 'desc' ? 'descending' : 'ascending'}
				>
					<button
						type="button"
						class="inline-flex min-h-11 cursor-pointer items-center gap-1 border-0 bg-transparent p-0 font-semibold text-inherit"
						onclick={toggleOrder}
					>
						Timestamp
						{#if order === 'desc'}
							<ArrowDownIcon aria-hidden="true" class="size-4" />
						{:else}
							<ArrowUpIcon aria-hidden="true" class="size-4" />
						{/if}
					</button>
				</th>
			</tr>
		</thead>
		<tbody>
			{#each entries as entry (entry.id)}
				<tr>
					<td data-label="Object">{entry.object_id}</td>
					<td data-label="Action">{entry.action}</td>
					<td data-label="Actor">{auditActorLabel(entry)}</td>
					<td data-label="Caller">{entry.caller ?? ''}</td>
					<td data-label="IP">{entry.ip}</td>
					<td data-label="Timestamp">
						<time datetime={entry.timestamp} title={entry.timestamp}>
							{formatTimestamp(entry.timestamp)}
						</time>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>

	<div role="group" aria-label="Query with curl" class="mt-4 space-y-2">
		<pre class="m-0 font-mono text-xs break-words whitespace-pre-wrap">{curlCommand}</pre>
		<Button variant="outline" class="min-h-11" onclick={() => copier.copy(true, curlCommand)}>
			{#if copied}
				<CheckIcon aria-hidden="true" />
			{:else}
				<CopyIcon aria-hidden="true" />
			{/if}
			{copied ? 'Copied' : 'Copy command'}
		</Button>
	</div>

	<CursorPager
		count={entries.length}
		canNewer={pager.canNewer}
		canOlder={pager.canOlder}
		onNewer={() => step(pager.newer)}
		onOlder={() => step(pager.older)}
		label="Audit log pages"
		{loading}
	/>
</main>
