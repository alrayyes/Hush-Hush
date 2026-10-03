export const COPIED_MS = 1500;

// Copy-with-feedback shared by every page with a Copy button. `onChange`
// receives the key just copied, then null once COPIED_MS has passed; a
// newer copy replaces an older one and restarts the window. A rejected
// write reports nothing, so there's never a false "Copied".
export function createCopier<K>(onChange: (copied: K | null) => void) {
	let timer: ReturnType<typeof setTimeout> | undefined;

	return {
		async copy(key: K, text: string) {
			try {
				await navigator.clipboard.writeText(text);
			} catch {
				return;
			}
			clearTimeout(timer);
			onChange(key);
			timer = setTimeout(() => onChange(null), COPIED_MS);
		},
		dispose() {
			clearTimeout(timer);
		},
	};
}
