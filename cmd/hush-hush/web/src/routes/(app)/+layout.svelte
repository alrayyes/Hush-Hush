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
		align-items: center;
		gap: 1rem;
		padding: 1rem;
		border-bottom: 1px solid #ddd;
	}

	nav button {
		margin-left: auto;
	}
</style>
