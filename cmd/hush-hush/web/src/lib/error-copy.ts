// What an error page says: plain words, no internals. The status picks the
// wording; the server's own message is never shown, since an unexpected
// failure's message can carry detail a visitor shouldn't see.
export type ErrorCopy = { title: string; message: string };

export function errorCopy(status: number): ErrorCopy {
	if (status === 404) {
		return {
			title: 'Page not found',
			message:
				"We couldn't find that page. The link may be mistyped, or the page may have moved.",
		};
	}

	return {
		title: 'Something went wrong',
		message:
			'That did not work, and it was not something you did. Try again in a moment, or head back to where you were.',
	};
}
