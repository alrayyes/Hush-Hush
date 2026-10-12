import type { PatchObjectRequest } from './api';

export type ObjectDetails = {
	slug: string;
	description?: string;
	tags: string[];
};

// detailChanges is what a details edit sends: only the fields that differ
// from what the form opened with, so an untouched field is left alone by
// the API. undefined means there is nothing to send. An emptied description
// or tag list is a change (the API clears on '' and []).
export function detailChanges(
	initial: ObjectDetails,
	draft: ObjectDetails,
): PatchObjectRequest | undefined {
	const changes: PatchObjectRequest = {};

	if (draft.slug !== initial.slug) {
		changes.slug = draft.slug;
	}

	if ((draft.description ?? '') !== (initial.description ?? '')) {
		changes.description = draft.description ?? '';
	}

	if (draft.tags.join(',') !== initial.tags.join(',')) {
		changes.tags = draft.tags;
	}

	return Object.keys(changes).length > 0 ? changes : undefined;
}

// needsReseal is whether an edit has to seal the value again: a typed value,
// a changed consumer list or a readable copy. Id, description and tags alone
// are a PATCH, which leaves the ciphertext (and the passkey touch) out of it.
// The value is the already-normalised text, '' when none was typed.
export function needsReseal(edit: {
	value: string;
	keepReadableCopy: boolean;
	usedBy: string[];
	initialUsedBy: string[];
}): boolean {
	return (
		edit.value !== '' ||
		edit.keepReadableCopy ||
		edit.usedBy.join(',') !== edit.initialUsedBy.join(',')
	);
}
