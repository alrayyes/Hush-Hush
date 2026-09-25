// @vitest-environment jsdom
//
// The only spec in this project that needs a real `document` global -
// registerWebMCPTools reads document.modelContext directly, and every
// other *.spec.ts here tests pure functions under the default node
// environment.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type ObjectMetadata } from './api';

const { listObjects } = vi.hoisted(() => ({
	listObjects: vi.fn(),
}));

vi.mock('./api', async (importOriginal) => {
	const actual = await importOriginal<typeof import('./api')>();

	return { ...actual, listObjects };
});

afterEach(() => {
	delete document.modelContext;
	vi.resetAllMocks();
});

describe('registerWebMCPTools', () => {
	it('is a no-op when document.modelContext is undefined', async () => {
		const { registerWebMCPTools } = await import('./webmcp');

		await expect(registerWebMCPTools()).resolves.toBeUndefined();
		expect(listObjects).not.toHaveBeenCalled();
	});

	it('registers exactly list_objects and get_object_metadata', async () => {
		const registerTool = vi.fn().mockResolvedValue(undefined);
		document.modelContext = { registerTool };

		const { registerWebMCPTools } = await import('./webmcp');
		await registerWebMCPTools();

		expect(registerTool).toHaveBeenCalledTimes(2);
		const names = registerTool.mock.calls.map(([tool]) => tool.name);
		expect(names).toEqual(['list_objects', 'get_object_metadata']);
	});
});

describe('the list_objects tool', () => {
	it("calls listObjects with the caller's used_by filter", async () => {
		const registerTool = vi.fn().mockResolvedValue(undefined);
		document.modelContext = { registerTool };
		const objects: ObjectMetadata[] = [
			{ id: 'a', used_by: ['homelab/vps-docker'] },
		];
		listObjects.mockResolvedValue(objects);

		const { registerWebMCPTools } = await import('./webmcp');
		await registerWebMCPTools();

		const listTool = registerTool.mock.calls.find(
			([tool]) => tool.name === 'list_objects',
		)?.[0];
		const result = await listTool.execute({ used_by: 'homelab/vps-docker' });

		expect(listObjects).toHaveBeenCalledWith('homelab/vps-docker');
		expect(result).toEqual(objects);
	});

	it('surfaces an ApiError as a clear Error', async () => {
		const registerTool = vi.fn().mockResolvedValue(undefined);
		document.modelContext = { registerTool };
		listObjects.mockRejectedValue(
			new ApiError(401, 'missing or invalid bearer token or session'),
		);

		const { registerWebMCPTools } = await import('./webmcp');
		await registerWebMCPTools();

		const listTool = registerTool.mock.calls.find(
			([tool]) => tool.name === 'list_objects',
		)?.[0];

		await expect(listTool.execute({})).rejects.toThrow(
			'list_objects: missing or invalid bearer token or session',
		);
	});
});

describe('the get_object_metadata tool', () => {
	it('resolves one object by id from the full list', async () => {
		const registerTool = vi.fn().mockResolvedValue(undefined);
		document.modelContext = { registerTool };
		const objects: ObjectMetadata[] = [
			{ id: 'mattermost_deploy_webhook', description: 'prod deploy webhook' },
			{ id: 'other' },
		];
		listObjects.mockResolvedValue(objects);

		const { registerWebMCPTools } = await import('./webmcp');
		await registerWebMCPTools();

		const getTool = registerTool.mock.calls.find(
			([tool]) => tool.name === 'get_object_metadata',
		)?.[0];
		const result = await getTool.execute({ id: 'mattermost_deploy_webhook' });

		expect(listObjects).toHaveBeenCalledWith();
		expect(result).toEqual(objects[0]);
	});

	it('throws a clear error for an unknown id', async () => {
		const registerTool = vi.fn().mockResolvedValue(undefined);
		document.modelContext = { registerTool };
		listObjects.mockResolvedValue([]);

		const { registerWebMCPTools } = await import('./webmcp');
		await registerWebMCPTools();

		const getTool = registerTool.mock.calls.find(
			([tool]) => tool.name === 'get_object_metadata',
		)?.[0];

		await expect(getTool.execute({ id: 'does-not-exist' })).rejects.toThrow(
			'get_object_metadata: unknown object: does-not-exist',
		);
	});
});
