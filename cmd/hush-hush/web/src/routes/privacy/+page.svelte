<svelte:head>
	<title>Privacy - hush-hush</title>
</svelte:head>

<main>
	<h1>Privacy</h1>
	<p>
		hush-hush sends no telemetry or analytics anywhere. Everything this
		page and its audit log record - object ids, timestamps, source IPs,
		self-reported caller labels, verified token and session identities -
		stays in this instance's own database, under whoever's running it.
		Secret values themselves are opaque ciphertext this service never
		decrypts.
	</p>
</main>

<style>
	main {
		max-width: 40rem;
		margin: 2rem auto;
		padding: 0 1rem;
	}
</style>
