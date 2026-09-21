// Theme preference: stored choice, then OS preference, then light -
// design.md's "Preference cascade on load" decision (#281). The same
// cascade also lives, duplicated in plain JS, as app.html's inline
// theme-init script - that copy has to run before this module's bundle
// loads at all, so it can't import from here; keep the two in sync by
// hand if this logic ever changes.

export const THEME_STORAGE_KEY = 'theme';

export type Theme = 'light' | 'dark';

function isTheme(value: string | null): value is Theme {
	return value === 'light' || value === 'dark';
}

export function resolveTheme(
	stored: string | null,
	prefersDark: boolean,
): Theme {
	if (isTheme(stored)) {
		return stored;
	}

	return prefersDark ? 'dark' : 'light';
}

// readStoredTheme/storeTheme/applyTheme wrap the browser APIs the toggle
// needs - kept out of the component so the cascade logic above stays
// testable with no DOM.

export function readStoredTheme(): Theme | null {
	try {
		const stored = localStorage.getItem(THEME_STORAGE_KEY);

		return isTheme(stored) ? stored : null;
	} catch {
		return null;
	}
}

export function storeTheme(theme: Theme): void {
	try {
		localStorage.setItem(THEME_STORAGE_KEY, theme);
	} catch {
		// Private-browsing/blocked storage: the toggle still works for the
		// rest of this page load, it just won't survive a reload.
	}
}

export function applyTheme(theme: Theme): void {
	document.documentElement.setAttribute('data-theme', theme);
}
