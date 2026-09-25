<script lang="ts">
// Nav markup and logout live in the shared root layout now
// (web-ui-design-system/tasks.md #2.1) - +layout.ts's redirect-when-
// unauthenticated guard is the only thing this layout still owns.
import { onMount } from 'svelte';
import { registerWebMCPTools } from '$lib/webmcp';

let { children } = $props();

// alrayyes/hush-hush#347: registered here, not the root layout - both
// tools need a session (they call listObjects, which 401s without one),
// and this layout only ever renders once +layout.ts's own guard has
// already confirmed one exists. A silent no-op wherever
// document.modelContext doesn't exist (docs/adr/0020-webmcp-tools.md).
onMount(() => {
	registerWebMCPTools();
});
</script>

{@render children()}
