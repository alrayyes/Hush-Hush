<script lang="ts">
let { version }: { version: string } = $props();
</script>

<footer>
	<a href="/changelog">{version}</a>
	<span aria-hidden="true">&middot;</span>
	<a href="/disclaimer">Disclaimer</a>
	<span aria-hidden="true">&middot;</span>
	<a href="/privacy">Privacy</a>
	<span aria-hidden="true">&middot;</span>
	<span>GPL-3.0</span>
</footer>

<style>
	footer {
		max-width: 60rem;
		margin: 3rem auto 1.5rem;
		padding: 0 1rem;
		font-size: 0.85rem;
		color: #666;
		display: flex;
		justify-content: center;
		gap: 0.5rem;
		align-items: center;
	}

	footer a {
		color: inherit;
	}
</style>
