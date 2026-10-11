// Attributes for a field a secret's plaintext is typed or pasted into
// (alrayyes/hush-hush#661). Chrome and Edge's enhanced spell check uploads
// field text, so these opt out of it and of autofill and autocorrect.
// Spread, not written inline: Svelte's textarea types don't list the
// non-standard `autocorrect`, and a spread skips the excess-property check.
export const valueFieldAttributes = {
	spellcheck: false,
	autocomplete: 'off',
	autocapitalize: 'off',
	autocorrect: 'off',
} as const;

// normalizeValue is what gets sealed: the typed text without the spaces,
// tabs and newlines a paste tends to carry at either end. Whitespace inside
// the value is part of it and stays. The server only ever sees ciphertext,
// so this can only happen in the browser, before sealing.
export function normalizeValue(value: string): string {
	return value.trim();
}
