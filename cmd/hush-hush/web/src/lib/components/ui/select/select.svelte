<script lang="ts">
import { Select as SelectPrimitive } from 'bits-ui';
import { provideSelectContext } from './select-context.svelte.js';

let {
	open = $bindable(false),
	value = $bindable(),
	...restProps
}: SelectPrimitive.RootProps = $props();

const uid = $props.id();
provideSelectContext(`${uid}-listbox`);
</script>

<SelectPrimitive.Root bind:open bind:value={value as never} {...restProps} />
