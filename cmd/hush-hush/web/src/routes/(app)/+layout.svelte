<script lang="ts">
// Nav markup and logout live in the shared root layout now
// (web-ui-design-system/tasks.md #2.1) - +layout.ts's redirect-when-
// unauthenticated guard is the only thing this layout still owns.
let { children } = $props();
</script>

{@render children()}
