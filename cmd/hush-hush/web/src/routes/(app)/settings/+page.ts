import { listConsumerTokens, listCredentials, listTokens } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ depends }) => {
	depends('app:settings');

	const [credentials, tokens, consumerTokens] = await Promise.all([
		listCredentials(),
		listTokens(),
		listConsumerTokens(),
	]);

	return { credentials, tokens, consumerTokens };
};
