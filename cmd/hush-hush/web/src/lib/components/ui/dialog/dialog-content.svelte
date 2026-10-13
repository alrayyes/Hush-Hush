<script lang="ts">
import XIcon from '@lucide/svelte/icons/x';
import { Dialog as DialogPrimitive } from 'bits-ui';
import type { ComponentProps, Snippet } from 'svelte';
import { Button } from '#lib/components/ui/button/index.js';
import { cn, type WithoutChildrenOrChild } from '#lib/utils.js';
import DialogPortal from './dialog-portal.svelte';
import * as Dialog from './index.js';

let {
	ref = $bindable(null),
	class: className,
	portalProps,
	children,
	showCloseButton = true,
	scroll = true,
	...restProps
}: WithoutChildrenOrChild<DialogPrimitive.ContentProps> & {
	portalProps?: WithoutChildrenOrChild<ComponentProps<typeof DialogPortal>>;
	children: Snippet;
	showCloseButton?: boolean;
	// A dialog that pins its own header and footer passes false and lays its
	// children out itself.
	scroll?: boolean;
} = $props();
</script>

<DialogPortal {...portalProps}>
	<Dialog.Overlay />
	<DialogPrimitive.Content
		bind:ref
		data-slot="dialog-content"
		class={cn(
			"bg-popover text-popover-foreground data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 data-closed:zoom-out-95 data-open:zoom-in-95 ring-foreground/10 flex max-h-[calc(100dvh_-_2rem)] max-w-[calc(100%_-_2rem)] flex-col overflow-hidden rounded-xl text-sm ring-1 duration-100 sm:max-w-sm fixed top-1/2 left-1/2 z-50 w-full -translate-x-1/2 -translate-y-1/2 outline-none",
			className
		)}
		{...restProps}
	>
		{#if scroll}
			<div class="grid min-h-0 gap-4 overflow-y-auto p-4">
				{@render children?.()}
			</div>
		{:else}
			{@render children?.()}
		{/if}
		{#if showCloseButton}
			<DialogPrimitive.Close data-slot="dialog-close">
				{#snippet child({ props })}
					<Button variant="ghost" class="absolute top-2 right-2" size="icon-sm" {...props}>
						<XIcon  />
						<span class="sr-only">Close</span>
					</Button>
				{/snippet}
			</DialogPrimitive.Close>
		{/if}
	</DialogPrimitive.Content>
</DialogPortal>
