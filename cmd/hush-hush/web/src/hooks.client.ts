import type { HandleClientError } from '@sveltejs/kit/hooks';

// An unexpected error (a load that throws, a render failure) is logged here
// and shown as one generic message: the error's own text can carry detail a
// visitor shouldn't see, and +error.svelte doesn't print it anyway.
export const handleError: HandleClientError = ({ error }) => {
	console.error('unexpected error', error);

	return { message: 'Something went wrong' };
};
