// Writes src/lib/api-limits.ts from the limits api/openapi.yaml declares.
// openapi-typescript emits types only and drops `maximum`/`default`, so a
// literal the front end needs (a page cap) would otherwise be copied by
// hand and drift. Run through `bun run generate:api`, or with --check
// (`bun run generate:api:check`) to fail on a stale file without writing.

type Parameter = {
	name?: string;
	schema?: { maximum?: number };
};

const specPath = new URL('../../../../api/openapi.yaml', import.meta.url);
const spec = Bun.YAML.parse(await Bun.file(specPath).text()) as {
	paths: Record<string, { get?: { parameters?: Parameter[] } }>;
};

const pageSize = spec.paths['/consumers']?.get?.parameters?.find(
	(parameter) => parameter.name === 'page_size',
);
const max = pageSize?.schema?.maximum;

if (typeof max !== 'number') {
	throw new Error('GET /consumers page_size has no maximum in the spec');
}

const out = new URL('../src/lib/api-limits.ts', import.meta.url);

const generated = `// Generated from api/openapi.yaml by scripts/generate-api-limits.ts.
// Do not edit by hand; run \`bun run generate:api\`.

// GET /consumers: the most consumers one page may ask for.
export const CONSUMERS_PAGE_SIZE_MAX = ${max};
`;

if (process.argv.includes('--check')) {
	if ((await Bun.file(out).text()) !== generated) {
		console.error('src/lib/api-limits.ts is stale: run `bun run generate:api`');
		process.exit(1);
	}
} else {
	await Bun.write(out, generated);
}
