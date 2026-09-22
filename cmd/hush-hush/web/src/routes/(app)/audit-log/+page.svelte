<script lang="ts">
import {
	ApiError,
	type AuditLogEntry,
	type AuditLogQuery,
	queryAuditLog,
} from '$lib/api';
import { auditActorLabel, toCSV, toJSON } from '$lib/audit-export';
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

// Applied filters - object/actor/caller are plain text, from/to are
// datetime-local strings converted to RFC 3339 on request. Removing a
// chip (or changing a field) refetches immediately, no "Apply" step
// (design.md's "Audit log UI" decision).
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
		class="mb-4 grid grid-cols-[repeat(auto-fit,minmax(10rem,1fr))] items-end gap-x-4 gap-y-2"
		onsubmit={(event) => {
			event.preventDefault();
			resetToFirstPage();
		}}
	>
		<label class="block text-sm" for="filter-object">Object id</label>
		<input id="filter-object" class="w-full" bind:value={objectFilter} onchange={resetToFirstPage} />

		<label class="block text-sm" for="filter-actor">Actor</label>
		<input id="filter-actor" class="w-full" bind:value={actorFilter} onchange={resetToFirstPage} />

		<label class="block text-sm" for="filter-caller">Caller</label>
		<input id="filter-caller" class="w-full" bind:value={callerFilter} onchange={resetToFirstPage} />

		<label class="block text-sm" for="filter-from">From</label>
		<input
			id="filter-from"
			class="w-full"
			type="datetime-local"
			bind:value={fromFilter}
			onchange={resetToFirstPage}
		/>

		<label class="block text-sm" for="filter-to">To</label>
		<input
			id="filter-to"
			class="w-full"
			type="datetime-local"
			bind:value={toFilter}
			onchange={resetToFirstPage}
		/>
	</form>

	{#if objectFilter || actorFilter || callerFilter || fromFilter || toFilter}
		<ul class="mb-4 flex list-none flex-wrap gap-2 p-0">
			{#if objectFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					object: {objectFilter}
					<button
						type="button"
						class="border-0 bg-transparent"
						onclick={() => clearFilter('object')}
						aria-label="Remove object filter"
					>
						&times;
					</button>
				</li>
			{/if}
			{#if actorFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					actor: {actorFilter}
					<button
						type="button"
						class="border-0 bg-transparent"
						onclick={() => clearFilter('actor')}
						aria-label="Remove actor filter"
					>
						&times;
					</button>
				</li>
			{/if}
			{#if callerFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					caller: {callerFilter}
					<button
						type="button"
						class="border-0 bg-transparent"
						onclick={() => clearFilter('caller')}
						aria-label="Remove caller filter"
					>
						&times;
					</button>
				</li>
			{/if}
			{#if fromFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					from: {fromFilter}
					<button
						type="button"
						class="border-0 bg-transparent"
						onclick={() => clearFilter('from')}
						aria-label="Remove from filter"
					>
						&times;
					</button>
				</li>
			{/if}
			{#if toFilter}
				<li class="rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					to: {toFilter}
					<button
						type="button"
						class="border-0 bg-transparent"
						onclick={() => clearFilter('to')}
						aria-label="Remove to filter"
					>
						&times;
					</button>
				</li>
			{/if}
		</ul>
	{/if}

	<div class="mb-4 flex gap-2">
		<button type="button" onclick={exportCSV}>Export CSV</button>
		<button type="button" onclick={exportJSON}>Export JSON</button>
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
		<button type="button" onclick={previousPage} disabled={cursorStack.length === 0 || loading}>
			Previous
		</button>
		<button type="button" onclick={nextPage} disabled={entries.length === 0 || loading}>
			Next
		</button>
	</div>
</main>
