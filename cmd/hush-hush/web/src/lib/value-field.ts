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
