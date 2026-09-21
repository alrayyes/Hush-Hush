<script lang="ts">
import '../app.css';
import { goto, invalidate } from '$app/navigation';
import { logout } from '$lib/api';
import favicon from '$lib/assets/favicon.svg';
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
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="topbar">
	{#if data.authenticated}
		<nav>
			<a href="/">Secrets</a>
			<a href="/consumers">Consumers</a>
			<a href="/audit-log">Audit log</a>
			<a href="/settings">Settings</a>
		</nav>
	{/if}
	<div class="topbar-actions">
		<ThemeToggle />
		{#if data.authenticated}
			<button type="button" onclick={handleLogout}>Log out</button>
		{/if}
	</div>
</div>

{@render children()}

<Footer version={data.version} />

<style>
	/* Mobile-first: nav (links only) and topbar-actions (theme toggle +
	   Log out, a separate account-actions group, not navigation) each get
	   their own full-width row - align-items: stretch, the flex column
	   default - instead of every control fighting over one shared line
	   via flex-wrap and margin-left: auto (#294,
	   alrayyes/hush-hush#294). min-width restores the single-row desktop
	   bar, nav's own flex: 1 pushing topbar-actions to the far end. */
	.topbar {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		border-bottom: 1px solid var(--color-border);
	}

	nav {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}

	.topbar-actions {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	@media (min-width: 40rem) {
		.topbar {
			flex-direction: row;
			align-items: center;
			gap: var(--space-4);
		}

		nav {
			flex: 1;
			gap: var(--space-4);
		}
	}
</style>
