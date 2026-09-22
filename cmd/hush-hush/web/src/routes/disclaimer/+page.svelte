<script lang="ts">
import { Card, CardContent } from '$lib/components/ui/card/index.js';
</script>

<svelte:head>
	<title>Disclaimer - hush-hush</title>
</svelte:head>

<main class="mx-auto my-8 max-w-160 px-4">
	<h1>Disclaimer</h1>
	<Card>
		<CardContent>
			<p>
				hush-hush is self-hosted software: whoever runs this instance operates
				it, holds the encryption keys, and is responsible for its data and
				availability. The project provides no hosted service, no support
				guarantee, and no warranty of any kind, per its GPL-3.0 licence.
			</p>
		</CardContent>
	</Card>
</main>
