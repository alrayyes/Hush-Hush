<script lang="ts">
import { goto } from '$app/navigation';
import { logout } from '$lib/api';

let { children } = $props();

async function handleLogout() {
	await logout();
	await goto('/login');
}
</script>

<nav>
	<a href="/">Secrets</a>
	<a href="/audit-log">Audit log</a>
	<a href="/settings">Settings</a>
	<button type="button" onclick={handleLogout}>Log out</button>
</nav>

{@render children()}

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
