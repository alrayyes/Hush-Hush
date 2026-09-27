// bytesToBase64 base64-encodes raw bytes, chunked to avoid spreading a
// large typed array as call arguments ("Maximum call stack size
// exceeded") - shared by api.ts's getObjectValue (decoding a fetched
// ciphertext) and sealing.ts's sealValue (encoding a freshly-sealed one),
// neither of which has a size limit this client can assume.
export function bytesToBase64(bytes: Uint8Array): string {
	let binary = '';
	const chunkSize = 0x8000;
	for (let i = 0; i < bytes.length; i += chunkSize) {
		binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
	}

	return btoa(binary);
}
