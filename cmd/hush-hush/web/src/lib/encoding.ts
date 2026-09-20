// utf8ToBase64 base64-encodes arbitrary text, UTF-8 safe - plain btoa()
// only accepts Latin1 and throws on anything outside it (an emoji, an
// accented character). Chunked the same way api.ts's own
// getObjectValue decodes a binary string, to avoid spreading a large
// array as call arguments.
export function utf8ToBase64(text: string): string {
	const bytes = new TextEncoder().encode(text);

	let binary = '';
	const chunkSize = 0x8000;
	for (let i = 0; i < bytes.length; i += chunkSize) {
		binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
	}

	return btoa(binary);
}
