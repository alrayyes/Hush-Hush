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
		padding: 0 var(--space-4);
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: var(--space-2);
		align-items: center;
	}

	footer a {
		color: inherit;
	}
</style>
