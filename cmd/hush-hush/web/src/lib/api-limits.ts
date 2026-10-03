// Generated from api/openapi.yaml by scripts/generate-api-limits.ts.
// Do not edit by hand; run `bun run generate:api`.

// GET /consumers: the most consumers one page may ask for.
export const CONSUMERS_PAGE_SIZE_MAX = 100;

// ttl_seconds on every token create and rotate: TokenTtlSeconds.
export const TOKEN_TTL_SECONDS_MIN = 1;
export const TOKEN_TTL_SECONDS_MAX = 31536000;
export const TOKEN_TTL_SECONDS_DEFAULT = 7776000;
