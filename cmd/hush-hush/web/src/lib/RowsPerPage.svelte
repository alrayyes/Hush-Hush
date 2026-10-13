<script lang="ts">
import * as Select from '#lib/components/ui/select/index.js';

// The "Rows per page" choice at the left of a pager bar
// (alrayyes/Hush-Hush#897). The trigger shows the current size itself, so the
// closed select reads correctly without the menu mounted. The label is a real
// element the trigger and the open list point at, not a placeholder. The trigger
// is a combobox: while the list is open it carries aria-activedescendant, which
// a plain button isn't allowed to have, and an open combobox must say which
// list it controls.
let {
	options,
	size,
	onchange,
	disabled = false,
}: {
	options: readonly number[];
	size: number;
	onchange: (size: number) => void;
	disabled?: boolean;
} = $props();

const uid = $props.id();
const labelId = `${uid}-rows-per-page`;
const listId = `${uid}-rows-listbox`;
</script>

<div class="flex items-center gap-2 text-sm text-muted-foreground">
	<span id={labelId}>Rows per page</span>
	<Select.Root
		type="single"
		value={String(size)}
		onValueChange={(value) => onchange(Number(value))}
		{disabled}
	>
		<Select.Trigger
			role="combobox"
			aria-labelledby={labelId}
			aria-controls={listId}
			class="h-11 min-w-20 text-foreground sm:h-8 sm:min-w-16"
		>
			{size}
		</Select.Trigger>
		<Select.Content id={listId} aria-labelledby={labelId}>
			{#each options as option (option)}
				<Select.Item value={String(option)} label={String(option)} />
			{/each}
		</Select.Content>
	</Select.Root>
</div>
