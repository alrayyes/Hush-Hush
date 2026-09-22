<script lang="ts">
import { Card, CardContent } from '$lib/components/ui/card/index.js';
</script>

<svelte:head>
	<title>Privacy - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-160 px-4">
	<h1>Privacy</h1>
	<Card>
		<CardContent>
			<p>
				hush-hush sends no telemetry or analytics anywhere. Everything this
				page and its audit log record - object ids, timestamps, source IPs,
				self-reported caller labels, verified token and session identities -
				stays in this instance's own database, under whoever's running it.
				Secret values themselves are opaque ciphertext this service never
				decrypts.
			</p>
		</CardContent>
	</Card>
</main>
