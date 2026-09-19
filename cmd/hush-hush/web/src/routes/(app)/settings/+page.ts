import { listCredentials, listTokens } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ depends }) => {
	depends('app:settings');

	const [credentials, tokens] = await Promise.all([
		listCredentials(),
		listTokens(),
	]);

	return { credentials, tokens };
};
