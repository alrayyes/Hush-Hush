<script lang="ts">
import { page } from '$app/state';
import '../app.css';
import { goto, invalidate } from '$app/navigation';
import { logout } from '$lib/api';
import favicon from '$lib/assets/favicon.svg';
import { Button } from '$lib/components/ui/button/index.js';
import Footer from '$lib/Footer.svelte';
import ThemeToggle from '$lib/ThemeToggle.svelte';
import type { LayoutData } from './$types';

let {
	data,
	children,
}: { data: LayoutData; children: import('svelte').Snippet } = $props();

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
	{ href: '/', label: 'Secrets' },
	{ href: '/consumers', label: 'Consumers' },
	{ href: '/audit-log', label: 'Audit log' },
	{ href: '/settings', label: 'Settings' },
];
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<!-- Mobile-first: nav (links only) and topbar-actions (theme toggle + Log
     out, a separate account-actions group, not navigation) each get their
     own full-width row instead of every control fighting over one shared
     line via flex-wrap and margin-left: auto (#294, alrayyes/hush-hush#294).
     sm: restores the single-row desktop bar, nav's own flex-1 pushing
     topbar-actions to the far end. -->
<div
	class="topbar flex flex-col gap-3 border-b border-border px-4 py-3 sm:flex-row sm:items-center sm:gap-4"
>
	{#if data.authenticated}
		<nav class="flex flex-wrap items-center gap-2 sm:flex-1 sm:gap-4">
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
	<div class="topbar-actions flex items-center gap-2">
		<ThemeToggle />
		{#if data.authenticated}
			<Button variant="outline" onclick={handleLogout}>Log out</Button>
		{/if}
	</div>
</div>

{@render children()}

<Footer version={data.version} />
