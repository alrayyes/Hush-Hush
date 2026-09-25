// WebMCP tool declarations (alrayyes/hush-hush#347) - read-only, metadata
// -only, feature-detected. document.modelContext doesn't exist in any
// browser without an origin-trial flag today; registerWebMCPTools is a
// silent no-op there, ready for when/if a browser ships this by default
// (docs/adr/0020-webmcp-tools.md).
//
// Both tools wrap the same listObjects() call the secrets overview page
// already uses - no new backend endpoint, no path to a secret's sealed
// value (getObjectValue is never called from here).

import { listObjects, type ObjectMetadata } from './api';

interface ListObjectsInput {
	used_by?: string;
}

interface GetObjectMetadataInput {
	id: string;
}

export async function registerWebMCPTools(): Promise<void> {
	const modelContext = document.modelContext;
	if (!modelContext) {
		return;
	}

	await modelContext.registerTool<ListObjectsInput, ObjectMetadata[]>({
		name: 'list_objects',
		description:
			"List every stored object's metadata (id, description, used_by) - never the sealed value. Optionally restricted to objects whose recorded used_by lineage includes a given consumer.",
		inputSchema: {
			type: 'object',
			properties: {
				used_by: {
					type: 'string',
					description:
						'Restrict to objects whose recorded used_by lineage includes this consumer.',
				},
			},
		},
		execute: (input) =>
			withToolErrors('list_objects', () => listObjects(input?.used_by)),
	});

	await modelContext.registerTool<GetObjectMetadataInput, ObjectMetadata>({
		name: 'get_object_metadata',
		description:
			"Look up one stored object's metadata (id, description, used_by) by id - never the sealed value.",
		inputSchema: {
			type: 'object',
			properties: {
				id: { type: 'string', description: "The object's id." },
			},
			required: ['id'],
		},
		execute: (input) =>
			withToolErrors('get_object_metadata', async () => {
				const objects = await listObjects();
				const found = objects.find((object) => object.id === input.id);
				if (!found) {
					throw new Error(`unknown object: ${input.id}`);
				}

				return found;
			}),
	});
}

// withToolErrors prefixes any error a tool's own logic throws - an
// ApiError from api.ts (which carries an HTTP status an in-browser agent
// has no use for) or a plain Error like get_object_metadata's "unknown
// object" - with the tool's own name, so an agent juggling more than one
// tool call can tell which one failed from the message alone.
async function withToolErrors<T>(
	tool: string,
	fn: () => Promise<T>,
): Promise<T> {
	try {
		return await fn();
	} catch (err) {
		const message = err instanceof Error ? err.message : String(err);
		throw new Error(`${tool}: ${message}`);
	}
}
