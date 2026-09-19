<script lang="ts">
import { goto } from '$app/navigation';
import { login } from '$lib/auth';

let pending = $state(false);
let error = $state('');

async function handleLogin() {
	pending = true;
	error = '';

	try {
		await login();
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
</script>

<svelte:head>
	<title>Log in - hush-hush</title>
</svelte:head>

<main class="login">
	<h1>hush-hush</h1>
	<p>Sign in with a passkey registered to this account.</p>

	<button type="button" onclick={handleLogin} disabled={pending}>
		{pending ? 'Waiting for your passkey…' : 'Log in with a passkey'}
	</button>

	{#if error}
		<p role="alert" class="error">{error}</p>
	{/if}
</main>

<style>
	.login {
		max-width: 24rem;
		margin: 4rem auto;
		padding: 0 1rem;
		text-align: center;
	}

	button {
		padding: 0.75rem 1.5rem;
		font-size: 1rem;
		cursor: pointer;
	}

	button:disabled {
		cursor: wait;
	}

	.error {
		color: #b00020;
	}
</style>
