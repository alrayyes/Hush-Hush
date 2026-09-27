<script lang="ts">
let { version }: { version: string } = $props();
</script>

<footer
	class="mx-auto mt-12 mb-6 flex max-w-content flex-wrap items-center justify-center gap-2 px-4 text-sm text-text-muted"
>
	<a href="/changelog" class="text-inherit">{version}</a>
	<span aria-hidden="true">&middot;</span>
	<a href="/disclaimer" class="text-inherit">Disclaimer</a>
	<span aria-hidden="true">&middot;</span>
	<a href="/privacy" class="text-inherit">Privacy</a>
	<span aria-hidden="true">&middot;</span>
	<a href="/license" class="text-inherit">GPL-3.0</a>
</footer>
