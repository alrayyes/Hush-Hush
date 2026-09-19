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

<main>
	<h1>Audit log</h1>

	<form
		class="filters"
		onsubmit={(event) => {
			event.preventDefault();
			resetToFirstPage();
		}}
	>
		<label for="filter-object">Object id</label>
		<input id="filter-object" bind:value={objectFilter} onchange={resetToFirstPage} />

		<label for="filter-actor">Actor</label>
		<input id="filter-actor" bind:value={actorFilter} onchange={resetToFirstPage} />

		<label for="filter-caller">Caller</label>
		<input id="filter-caller" bind:value={callerFilter} onchange={resetToFirstPage} />

		<label for="filter-from">From</label>
		<input
			id="filter-from"
			type="datetime-local"
			bind:value={fromFilter}
			onchange={resetToFirstPage}
		/>

		<label for="filter-to">To</label>
		<input id="filter-to" type="datetime-local" bind:value={toFilter} onchange={resetToFirstPage} />
	</form>

	{#if objectFilter || actorFilter || callerFilter || fromFilter || toFilter}
		<ul class="chips">
			{#if objectFilter}
				<li>
					object: {objectFilter}
					<button type="button" onclick={() => clearFilter('object')} aria-label="Remove object filter">
						&times;
					</button>
				</li>
			{/if}
			{#if actorFilter}
				<li>
					actor: {actorFilter}
					<button type="button" onclick={() => clearFilter('actor')} aria-label="Remove actor filter">
						&times;
					</button>
				</li>
			{/if}
			{#if callerFilter}
				<li>
					caller: {callerFilter}
					<button type="button" onclick={() => clearFilter('caller')} aria-label="Remove caller filter">
						&times;
					</button>
				</li>
			{/if}
			{#if fromFilter}
				<li>
					from: {fromFilter}
					<button type="button" onclick={() => clearFilter('from')} aria-label="Remove from filter">
						&times;
					</button>
				</li>
			{/if}
			{#if toFilter}
				<li>
					to: {toFilter}
					<button type="button" onclick={() => clearFilter('to')} aria-label="Remove to filter">
						&times;
					</button>
				</li>
			{/if}
		</ul>
	{/if}

	<div class="export">
		<button type="button" onclick={exportCSV}>Export CSV</button>
		<button type="button" onclick={exportJSON}>Export JSON</button>
	</div>

	{#if loadError}
		<p role="alert" class="error">{loadError}</p>
	{/if}

	<table aria-busy={loading}>
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
					<td>{entry.object_id}</td>
					<td>{entry.action}</td>
					<td>{auditActorLabel(entry)}</td>
					<td>{entry.caller ?? ''}</td>
					<td>{entry.timestamp}</td>
				</tr>
			{/each}
		</tbody>
	</table>

	<div class="pagination">
		<button type="button" onclick={previousPage} disabled={cursorStack.length === 0 || loading}>
			Previous
		</button>
		<button type="button" onclick={nextPage} disabled={entries.length === 0 || loading}>
			Next
		</button>
	</div>
</main>

<style>
	main {
		max-width: 70rem;
		margin: 2rem auto;
		padding: 0 1rem;
	}

	.filters {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
		gap: 0.5rem 1rem;
		align-items: end;
		margin-bottom: 1rem;
	}

	.filters label {
		display: block;
		font-size: 0.85rem;
	}

	.filters input {
		width: 100%;
		box-sizing: border-box;
	}

	.chips {
		list-style: none;
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		padding: 0;
		margin-bottom: 1rem;
	}

	.chips li {
		background: #eee;
		border-radius: 1rem;
		padding: 0.25rem 0.5rem;
		font-size: 0.85rem;
	}

	.chips button {
		background: none;
		border: none;
		cursor: pointer;
	}

	.export {
		margin-bottom: 1rem;
		display: flex;
		gap: 0.5rem;
	}

	.error {
		color: #b00020;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	th,
	td {
		text-align: left;
		padding: 0.5rem;
		border-bottom: 1px solid #ddd;
	}

	.pagination {
		display: flex;
		gap: 0.5rem;
		margin-top: 1rem;
	}
</style>
