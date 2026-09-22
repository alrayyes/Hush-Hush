<script lang="ts">
import {
	ApiError,
	type AuditLogEntry,
	type AuditLogQuery,
	queryAuditLog,
} from '$lib/api';
import { auditActorLabel, toCSV, toJSON } from '$lib/audit-export';
import { Button } from '$lib/components/ui/button/index.js';
import { Label } from '$lib/components/ui/label/index.js';
import * as Select from '$lib/components/ui/select/index.js';
import type { PageData } from './$types';

let { data }: { data: PageData } = $props();

// Seeded once from the load function's own unfiltered first page, then
// owned entirely by fetchPage below - this page never calls
// invalidate(), so there's nothing for `data` itself to update again
// with; every filter/pagination change refetches directly instead.
// svelte-check's state_referenced_locally warning is exactly this
// intentional one-time seed, not a sync bug.
let entries: AuditLogEntry[] = $state(data.entries);
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

// cursorStack holds the `after` value that produced each earlier page,
// so "Previous" can pop back to it - the API itself only ever pages
// forward (design.md's "Audit log UI" decision: id-based cursor, no
// offset), so "previous" is this page's own bookkeeping, not a second
// server capability.
let cursorStack: (number | undefined)[] = $state([]);
let currentAfter: number | undefined = $state(undefined);

function currentQuery(after: number | undefined): AuditLogQuery {
	return {
		object_id: objectFilter || undefined,
		actor: actorFilter || undefined,
		caller: callerFilter || undefined,
		from: fromFilter ? new Date(fromFilter).toISOString() : undefined,
		to: toFilter ? new Date(toFilter).toISOString() : undefined,
		after,
	};
}

async function fetchPage(after: number | undefined) {
	loading = true;
	loadError = '';

	try {
		entries = await queryAuditLog(currentQuery(after));
		currentAfter = after;
	} catch (err) {
		loadError =
			err instanceof ApiError ? err.message : 'Failed to load the audit log.';
	} finally {
		loading = false;
	}
}

function resetToFirstPage() {
	cursorStack = [];
	void fetchPage(undefined);
}

function nextPage() {
	if (entries.length === 0) return;
	cursorStack = [...cursorStack, currentAfter];
	void fetchPage(entries[entries.length - 1].id);
}

function previousPage() {
	if (cursorStack.length === 0) return;
	const prev = cursorStack[cursorStack.length - 1];
	cursorStack = cursorStack.slice(0, -1);
	void fetchPage(prev);
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
			<input
				id="filter-from"
				class="w-full"
				type="datetime-local"
				bind:value={fromFilter}
				onchange={resetToFirstPage}
			/>
		</div>

		<div class="flex flex-col gap-1">
			<label class="text-sm" for="filter-to">To</label>
			<input
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
		<Button variant="outline" onclick={exportCSV}>Export CSV</Button>
		<Button variant="outline" onclick={exportJSON}>Export JSON</Button>
	</div>

	{#if loadError}
		<p role="alert" class="text-error">{loadError}</p>
	{/if}

	<table class="responsive-table" aria-busy={loading}>
		<thead>
			<tr>
				<th scope="col">Object</th>
				<th scope="col">Action</th>
				<th scope="col">Actor</th>
				<th scope="col">Caller</th>
				<th scope="col">Timestamp</th>
			</tr>
		</thead>
		<tbody>
			{#each entries as entry (entry.id)}
				<tr>
					<td data-label="Object">{entry.object_id}</td>
					<td data-label="Action">{entry.action}</td>
					<td data-label="Actor">{auditActorLabel(entry)}</td>
					<td data-label="Caller">{entry.caller ?? ''}</td>
					<td data-label="Timestamp">{entry.timestamp}</td>
				</tr>
			{/each}
		</tbody>
	</table>

	<div class="mt-4 flex gap-2">
		<Button
			variant="outline"
			onclick={previousPage}
			disabled={cursorStack.length === 0 || loading}
		>
			Previous
		</Button>
		<Button variant="outline" onclick={nextPage} disabled={entries.length === 0 || loading}>
			Next
		</Button>
	</div>
</main>
