<script lang="ts">
import { goto } from '$app/navigation';
import { login, registerPasskey } from '$lib/auth';

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

// The server accepts an anonymous registration only when no admin
// account exists yet (auth/spec.md's "Registering a first passkey"
// scenario) - registering another one afterward is settings' own job,
// behind a real session. This is deliberately the secondary action
// here, not the primary one: it's the bootstrap path for a fresh
// install, not a second way to log in (alrayyes/hush-hush#237 - before
// this, there was no way to reach it at all on a fresh install).
async function handleRegister() {
	pending = true;
	error = '';

	try {
		await registerPasskey();
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
	<p>Sign in with a passkey registered to this account.</p>

	<button type="button" onclick={handleLogin} disabled={pending}>
		{pending ? 'Waiting for your passkey…' : 'Log in with a passkey'}
	</button>

	{#if error}
		<p role="alert" class="error">{error}</p>
	{/if}

	<p class="register">
		No account yet?
		<button type="button" class="link" onclick={handleRegister} disabled={pending}>
			Register the first passkey
		</button>
	</p>
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

	.register {
		margin-top: 2rem;
		font-size: 0.85rem;
		color: #666;
	}

	.link {
		padding: 0;
		background: none;
		border: none;
		font: inherit;
		font-size: inherit;
		color: #0057b3;
		text-decoration: underline;
		cursor: pointer;
	}
</style>
