<script lang="ts">
import CheckIcon from '@lucide/svelte/icons/check';
import CopyIcon from '@lucide/svelte/icons/copy';
import { onDestroy } from 'svelte';
import { createCopier } from '#lib/clipboard.js';
import { Button } from '#lib/components/ui/button/index.js';
import { Textarea } from '#lib/components/ui/textarea/index.js';

// A value the server shows once (a new token, the recovery phrase): a
// read-only field that stays selectable, and a Copy button beside it.
// `label` names the field, `noun` names what the button copies ("token",
// "recovery phrase") so its accessible name says so.
let { value, label, noun }: { value: string; label: string; noun: string } =
	$props();

let state: 'idle' | 'copied' | 'failed' = $state('idle');
const copier = createCopier<boolean>(
	(copied) => (state = copied === null ? 'idle' : 'copied'),
	(failed) => (state = failed === null ? 'idle' : 'failed'),
);
onDestroy(copier.dispose);
</script>

<div class="space-y-2">
	<Textarea readonly rows={3} {value} aria-label={label} class="w-full" />
	<div class="flex items-center gap-2">
		<Button
			type="button"
			variant="outline"
			class="min-h-11 min-w-11"
			aria-label={state === 'copied'
				? `Copied ${noun}`
				: state === 'failed'
					? `Copy ${noun} failed`
					: `Copy ${noun}`}
			onclick={() => copier.copy(true, value)}
		>
			{#if state === 'copied'}
				<CheckIcon aria-hidden="true" />
			{:else}
				<CopyIcon aria-hidden="true" />
			{/if}
			{state === 'copied' ? 'Copied' : state === 'failed' ? 'Copy failed' : 'Copy'}
		</Button>
		{#if state === 'failed'}
			<span role="status" class="text-sm text-error">
				Couldn't copy. Select the value above and copy it by hand.
			</span>
		{/if}
	</div>
</div>
