<script lang="ts">
import { page } from '$app/state';
import '../app.css';
import LockIcon from '@lucide/svelte/icons/lock';
import ScrollTextIcon from '@lucide/svelte/icons/scroll-text';
import SettingsIcon from '@lucide/svelte/icons/settings';
import TerminalIcon from '@lucide/svelte/icons/terminal';
import UserIcon from '@lucide/svelte/icons/user';
import { goto, invalidate } from '$app/navigation';
import { logout } from '$lib/api';
import favicon from '$lib/assets/favicon.svg';
import { Button, buttonVariants } from '$lib/components/ui/button/index.js';
import * as Dialog from '$lib/components/ui/dialog/index.js';
import Footer from '$lib/Footer.svelte';
import ThemeToggle from '$lib/ThemeToggle.svelte';
import type { LayoutData } from './$types';

let {
	data,
	children,
}: { data: LayoutData; children: import('svelte').Snippet } = $props();

let accountOpen = $state(false);

async function handleLogout() {
	await logout();
	await invalidate('app:auth');
	await goto('/login');
}

// #301: the current page gets both a visual difference and
// aria-current="page" - either alone loses half the audience (a11y
// research linked from that issue). Exact pathname match, not a prefix
// one: /consumers?used_by=... and /consumers?page=2 both keep
// page.url.pathname === '/consumers', but nothing here nests routes
// under one another the way a prefix match would need to handle.
const navLinks = [
	{ href: '/', label: 'Secrets', icon: LockIcon },
	{ href: '/consumers', label: 'Consumers', icon: TerminalIcon },
	{ href: '/audit-log', label: 'Audit log', icon: ScrollTextIcon },
	{ href: '/settings', label: 'Settings', icon: SettingsIcon },
];
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<!-- #481: one banner at every width. Brand, optional instance label and
     status pill sit on the left; below md the account actions (theme toggle,
     Log out) collapse behind one account button so the bar stays on one or
     two rows at 320px. md+ keeps the nav and the actions inline. -->
<header
	class="topbar flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-border px-4 py-3"
>
	<div class="flex items-center gap-2">
		<span class="font-mono font-bold">HUSH-HUSH</span>
		{#if data.environment !== undefined}
			<span
				class="rounded border border-border px-1.5 py-0.5 text-xs font-semibold uppercase text-text-muted"
				>{data.environment}</span
			>
		{/if}
	</div>
	<span
		class={[
			'inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-semibold',
			data.healthy ? 'border-accent text-accent' : 'border-error text-error',
		]}
	>
		<span aria-hidden="true" class="size-2 rounded-full bg-current"></span>
		{data.healthy ? 'ACTIVE' : 'OFFLINE'}
	</span>
	{#if data.authenticated}
		<nav
			aria-label="Primary"
			class="hidden flex-1 flex-wrap items-center gap-2 md:flex md:gap-4"
		>
			{#each navLinks as link (link.href)}
				<Button
					href={link.href}
					variant="ghost"
					aria-current={page.url.pathname === link.href ? 'page' : undefined}
					class={page.url.pathname === link.href
						? 'h-auto rounded-none border-b-2 border-accent px-1 py-1 font-bold no-underline'
						: 'h-auto rounded-none border-b-2 border-transparent px-1 py-1 no-underline'}
				>
					{link.label}
				</Button>
			{/each}
		</nav>
	{/if}
	<div class="topbar-actions ml-auto hidden items-center gap-2 md:flex">
		<ThemeToggle />
		{#if data.authenticated}
			<Button variant="outline" onclick={handleLogout}>Log out</Button>
		{/if}
	</div>
	<div class="topbar-actions ml-auto flex items-center gap-2 md:hidden">
		{#if data.authenticated}
			<Dialog.Root bind:open={accountOpen}>
				<Dialog.Trigger
					aria-label="Account menu"
					class={buttonVariants({ variant: 'ghost', size: 'icon' }) +
						' min-h-11 min-w-11'}
				>
					<UserIcon aria-hidden="true" />
				</Dialog.Trigger>
				<Dialog.Content>
					<Dialog.Header>
						<Dialog.Title>Account</Dialog.Title>
					</Dialog.Header>
					<div class="flex items-center gap-2">
						<ThemeToggle />
						<Button
							variant="outline"
							class="min-h-11"
							onclick={async () => {
								accountOpen = false;
								await handleLogout();
							}}>Log out</Button
						>
					</div>
				</Dialog.Content>
			</Dialog.Root>
		{:else}
			<ThemeToggle />
		{/if}
	</div>
</header>

{@render children()}

{#if data.authenticated}
	<!-- #480: below md the links move to a fixed bottom tab bar; the
	     wrapper's bottom padding keeps it from covering the footer. -->
	<div class="pb-20 md:pb-0">
		<Footer version={data.version} />
	</div>
	<nav
		aria-label="Primary (mobile)"
		class="fixed bottom-0 left-0 right-0 z-50 flex justify-around border-t border-border bg-surface pb-2 md:hidden"
	>
		{#each navLinks as link (link.href)}
			{@const active = page.url.pathname === link.href}
			<a
				href={link.href}
				aria-current={active ? 'page' : undefined}
				class={[
					'flex min-h-11 min-w-14 flex-col items-center justify-center text-xs no-underline',
					active ? 'font-bold text-accent' : 'text-text-muted',
				]}
			>
				<link.icon aria-hidden="true" class="size-5" />
				{link.label}
			</a>
		{/each}
	</nav>
{:else}
	<Footer version={data.version} />
{/if}
