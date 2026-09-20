<script lang="ts">
// A creatable combobox for picking consumer names - the ARIA APG
// combobox pattern (role="combobox" + role="listbox"/"option"), with
// one synthesized "Add <value>" option appended when the typed text
// matches nothing already offered. No client library: a combobox with
// one synthesized extra option is a small enough component to hand-
// write against the pattern directly
// (openspec/changes/consumer-combobox/design.md).
//
// Each selection appends to `value` (a list) rather than replacing it -
// used_by is multi-valued, so this is a tag picker built on the
// combobox pattern, not a single-value one.
import { listConsumers } from './api';

let { value = $bindable([]), id }: { value: string[]; id: string } = $props();

let known: string[] = $state([]);
let query = $state('');
let open = $state(false);
let activeIndex = $state(-1);
let input: HTMLInputElement | undefined = $state();

listConsumers()
	.then((consumers) => {
		known = consumers;
	})
	.catch(() => {
		// A failed lookup just means no suggestions - typing and adding a
		// new consumer still works with an empty known list.
	});

const trimmedQuery = $derived(query.trim());
const filtered = $derived(
	known.filter(
		(c) =>
			!value.includes(c) &&
			c.toLowerCase().includes(trimmedQuery.toLowerCase()),
	),
);
const offersAdd = $derived(
	trimmedQuery !== '' &&
		!known.includes(trimmedQuery) &&
		!value.includes(trimmedQuery),
);
const options = $derived(offersAdd ? [...filtered, trimmedQuery] : filtered);
const listboxId = $derived(`${id}-listbox`);

function addConsumer(name: string) {
	if (!value.includes(name)) {
		value = [...value, name];
	}

	query = '';
	activeIndex = -1;
	open = false;
	input?.focus();
}

function removeConsumer(name: string) {
	value = value.filter((v) => v !== name);
}

function onInput() {
	open = trimmedQuery !== '' || known.length > 0;
	activeIndex = -1;
}

function onKeydown(event: KeyboardEvent) {
	if (event.key === 'ArrowDown') {
		event.preventDefault();
		open = true;
		activeIndex = Math.min(activeIndex + 1, options.length - 1);
	} else if (event.key === 'ArrowUp') {
		event.preventDefault();
		activeIndex = Math.max(activeIndex - 1, 0);
	} else if (event.key === 'Enter') {
		event.preventDefault();
		if (activeIndex >= 0 && options[activeIndex]) {
			addConsumer(options[activeIndex]);
		} else if (trimmedQuery !== '') {
			addConsumer(trimmedQuery);
		}
	} else if (event.key === 'Escape') {
		open = false;
		activeIndex = -1;
	} else if (event.key === 'Backspace' && query === '' && value.length > 0) {
		value = value.slice(0, -1);
	}
}
</script>

<div class="combobox">
	{#if value.length > 0}
		<ul class="chips">
			{#each value as consumer (consumer)}
				<li>
					{consumer}
					<button
						type="button"
						onclick={() => removeConsumer(consumer)}
						aria-label={`Remove ${consumer}`}
					>
						&times;
					</button>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="input-wrapper">
		<input
			bind:this={input}
			{id}
			type="text"
			role="combobox"
			aria-expanded={open && options.length > 0}
			aria-controls={listboxId}
			aria-activedescendant={activeIndex >= 0 ? `${listboxId}-${activeIndex}` : undefined}
			aria-autocomplete="list"
			autocomplete="off"
			bind:value={query}
			oninput={onInput}
			onfocus={() => {
				open = true;
			}}
			onblur={() => {
				open = false;
			}}
			onkeydown={onKeydown}
			placeholder="Add a consumer"
		/>

		{#if open && options.length > 0}
			<ul id={listboxId} role="listbox" class="options">
				{#each options as option, i (option)}
					<li
						id={`${listboxId}-${i}`}
						role="option"
						aria-selected={i === activeIndex}
						class:active={i === activeIndex}
						onmousedown={(event) => {
							event.preventDefault();
							addConsumer(option);
						}}
					>
						{i === filtered.length && offersAdd ? `Add "${option}"` : option}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</div>

<style>
	.combobox {
		position: relative;
	}

	.chips {
		list-style: none;
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
		padding: 0;
		margin: 0 0 var(--space-2);
	}

	.chips li {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		background: var(--color-border-subtle);
		border-radius: 1rem;
		padding: var(--space-1) var(--space-2);
		font-size: var(--font-size-sm);
	}

	.chips button {
		padding: 0;
		border: none;
		background: none;
		font: inherit;
		line-height: 1;
		cursor: pointer;
	}

	.input-wrapper {
		position: relative;
	}

	.input-wrapper input {
		width: 100%;
	}

	.options {
		position: absolute;
		z-index: 1;
		top: calc(100% + var(--space-1));
		left: 0;
		right: 0;
		max-height: 12rem;
		overflow-y: auto;
		margin: 0;
		padding: var(--space-1) 0;
		list-style: none;
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: var(--radius);
	}

	.options li {
		padding: var(--space-2);
		cursor: pointer;
	}

	.options li.active {
		background: var(--color-border-subtle);
	}
</style>
