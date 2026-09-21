<script lang="ts">
import '../app.css';
import { goto, invalidate } from '$app/navigation';
import { logout } from '$lib/api';
import favicon from '$lib/assets/favicon.svg';
import Footer from '$lib/Footer.svelte';
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

{#if data.authenticated}
	<nav>
		<a href="/">Secrets</a>
		<a href="/consumers">Consumers</a>
		<a href="/audit-log">Audit log</a>
		<a href="/settings">Settings</a>
		<button type="button" onclick={handleLogout}>Log out</button>
	</nav>
{/if}

{@render children()}

<Footer version={data.version} />

<style>
	nav {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-4);
		padding: var(--space-4);
		border-bottom: 1px solid var(--color-border);
	}

	nav button {
		margin-left: auto;
	}
</style>
