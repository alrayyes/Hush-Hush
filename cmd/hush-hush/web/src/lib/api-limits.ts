// Generated from api/openapi.yaml by scripts/generate-api-limits.ts.
// Do not edit by hand; run `bun run generate:api`.

// GET /consumers: the most consumers one page may ask for.
export const CONSUMERS_PAGE_SIZE_MAX = 100;

// limit on GET /objects, /tokens, /consumer-tokens and /credentials: the most
// rows one page may ask for.
export const PAGE_LIMIT_MAX = 500;

// A single tag: Tag's pattern, which also fixes its length.
export const TAG_PATTERN = '^[a-z0-9._/-]{1,32}$';

// The most tags one object may carry: Tags' maxItems.
export const TAGS_MAX_ITEMS = 10;

// ttl_seconds on every token create and rotate: TokenTtlSeconds.
export const TOKEN_TTL_SECONDS_MIN = 1;
export const TOKEN_TTL_SECONDS_MAX = 31536000;
export const TOKEN_TTL_SECONDS_DEFAULT = 7776000;
