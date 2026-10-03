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
	components: {
		schemas: {
			Tag?: { pattern?: string };
			Tags?: { maxItems?: number };
			TokenTtlSeconds?: {
				minimum?: number;
				maximum?: number;
				default?: number;
			};
		};
	};
};

const pageSize = spec.paths['/consumers']?.get?.parameters?.find(
	(parameter) => parameter.name === 'page_size',
);
const max = pageSize?.schema?.maximum;

if (typeof max !== 'number') {
	throw new Error('GET /consumers page_size has no maximum in the spec');
}

const tagPattern = spec.components.schemas.Tag?.pattern;
const tagsMaxItems = spec.components.schemas.Tags?.maxItems;

if (typeof tagPattern !== 'string' || typeof tagsMaxItems !== 'number') {
	throw new Error('Tag has no pattern, or Tags no maxItems, in the spec');
}

// Biome's quote style for the generated file, so it needs no reformatting.
function singleQuoted(text: string): string {
	return text.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

const ttl = spec.components.schemas.TokenTtlSeconds;

if (
	typeof ttl?.minimum !== 'number' ||
	typeof ttl.maximum !== 'number' ||
	typeof ttl.default !== 'number'
) {
	throw new Error('TokenTtlSeconds needs a minimum, maximum and default');
}

const out = new URL('../src/lib/api-limits.ts', import.meta.url);

const generated = `// Generated from api/openapi.yaml by scripts/generate-api-limits.ts.
// Do not edit by hand; run \`bun run generate:api\`.

// GET /consumers: the most consumers one page may ask for.
export const CONSUMERS_PAGE_SIZE_MAX = ${max};

// A single tag: Tag's pattern, which also fixes its length.
export const TAG_PATTERN = '${singleQuoted(tagPattern)}';

// The most tags one object may carry: Tags' maxItems.
export const TAGS_MAX_ITEMS = ${tagsMaxItems};

// ttl_seconds on every token create and rotate: TokenTtlSeconds.
export const TOKEN_TTL_SECONDS_MIN = ${ttl.minimum};
export const TOKEN_TTL_SECONDS_MAX = ${ttl.maximum};
export const TOKEN_TTL_SECONDS_DEFAULT = ${ttl.default};
`;

if (process.argv.includes('--check')) {
	if ((await Bun.file(out).text()) !== generated) {
		console.error('src/lib/api-limits.ts is stale: run `bun run generate:api`');
		process.exit(1);
	}
} else {
	await Bun.write(out, generated);
}
