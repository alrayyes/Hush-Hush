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

<div class="relative">
	{#if value.length > 0}
		<ul class="m-0 mb-2 flex list-none flex-wrap gap-2 p-0">
			{#each value as consumer (consumer)}
				<li class="flex items-center gap-1 rounded-2xl bg-border-subtle px-2 py-1 text-sm">
					{consumer}
					<button
						type="button"
						class="border-0 bg-transparent p-0 text-sm font-normal leading-none"
						onclick={() => removeConsumer(consumer)}
						aria-label={`Remove ${consumer}`}
					>
						&times;
					</button>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="relative">
		<input
			bind:this={input}
			{id}
			type="text"
			class="w-full"
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
			<ul
				id={listboxId}
				role="listbox"
				class="absolute inset-x-0 top-[calc(100%+0.25rem)] z-10 m-0 max-h-48 list-none overflow-y-auto rounded border border-border bg-surface py-1"
			>
				{#each options as option, i (option)}
					<li
						id={`${listboxId}-${i}`}
						role="option"
						aria-selected={i === activeIndex}
						class="cursor-pointer p-2"
						class:bg-border-subtle={i === activeIndex}
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
