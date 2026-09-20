<script lang="ts">
import { onMount } from 'svelte';
import { goto, invalidate } from '$app/navigation';
import { getAuthStatus } from '$lib/api';
import { login, registerPasskey } from '$lib/auth';

let pending = $state(false);
let error = $state('');

// bootstrapped is undefined while the status check is in flight or has
// failed, so the login page shows neither action until it resolves -
// avoids flashing the wrong one, and avoids ever offering "register" to
// an already-bootstrapped install or "log in" to a fresh one with no way
// in (openspec/changes/gate-passkey-registration-ui/design.md's "Loading
// state" decision).
let bootstrapped = $state<boolean | undefined>(undefined);
let statusFailed = $state(false);

async function loadStatus() {
	statusFailed = false;

	try {
		bootstrapped = await getAuthStatus();
	} catch {
		statusFailed = true;
	}
}

onMount(loadStatus);

async function handleLogin() {
	pending = true;
	error = '';

	try {
		await login();
		await invalidate('app:auth');
		await goto('/');
	} catch {
		// web-ui/spec.md's "Failed ceremony shows an error, not a crash"
		// scenario - a rejected/cancelled browser prompt and a rejected
		// assertion both land here, and neither gets more detail than
		// this: the real reason (wrong key, unknown credential, a
		// cancelled prompt) isn't this visitor's to know.
		error =
			"Login failed. Check that you're using a registered passkey and try again.";
	} finally {
		pending = false;
	}
}

// The server accepts an anonymous registration only when no admin
// account exists yet (auth/spec.md's "Registering a first passkey"
// scenario) - registering another one afterward is settings' own job,
// behind a real session. Only reachable here while bootstrapped is
// false, since the page renders exactly one primary action
// (alrayyes/hush-hush#248).
async function handleRegister() {
	pending = true;
	error = '';

	try {
		await registerPasskey();
		await invalidate('app:auth');
		await goto('/');
	} catch {
		error =
			'Registration failed. If an account already exists, log in with an existing passkey instead.';
	} finally {
		pending = false;
	}
}
</script>

<svelte:head>
	<title>Log in - hush-hush</title>
</svelte:head>

<main class="login">
	<h1>hush-hush</h1>

	{#if statusFailed}
		<p role="alert" class="error">Couldn't reach the server. Try again.</p>
		<button type="button" onclick={loadStatus}>Retry</button>
	{:else if bootstrapped === undefined}
		<p>Checking account status…</p>
	{:else if bootstrapped}
		<p>Sign in with a passkey registered to this account.</p>

		<button type="button" onclick={handleLogin} disabled={pending}>
			{pending ? 'Waiting for your passkey…' : 'Log in with a passkey'}
		</button>
	{:else}
		<p>No account yet - register the first passkey to set one up.</p>

		<button type="button" onclick={handleRegister} disabled={pending}>
			{pending ? 'Waiting for your passkey…' : 'Register passkey'}
		</button>
	{/if}

	{#if error}
		<p role="alert" class="error">{error}</p>
	{/if}
</main>

<style>
	.login {
		max-width: 24rem;
		margin: 4rem auto;
		padding: 0 var(--space-4);
		text-align: center;
	}

	button {
		padding: var(--space-3) var(--space-6);
		font-size: var(--font-size-base);
		cursor: pointer;
	}

	button:disabled {
		cursor: wait;
	}

	.error {
		color: var(--color-error);
	}
</style>
