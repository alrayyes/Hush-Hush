# 32. The web UI is served under a Content-Security-Policy

## Status

Accepted. Adds to the web UI of [ADR 9](0009-sveltekit-static-adapter.md) and
[ADR 10](0010-go-embed-single-binary.md); changes nothing about how a value
is sealed.

## Context

The web UI seals a secret in the browser before it's sent
([ADR 1](0001-per-object-age-encryption.md)), so the server never sees
plaintext. That guarantee holds only if the page that does the sealing is the
page the server meant to serve. Before this, the server sent no security
headers, so a script injected into the page, or pulled in from a third party,
could read the value from the text field before it was sealed
([issue #660](https://github.com/alrayyes/hush-hush/issues/660), found in the
research on [#659](https://github.com/alrayyes/hush-hush/issues/659)).

## Decision

Every response from the static handler carries a `Content-Security-Policy`
header:

- `default-src 'self'`, `connect-src 'self'`, `font-src 'self'`,
  `img-src 'self' data:`
- `script-src 'self'` plus the SHA-256 hash of each inline script in the
  build's `index.html`. No `unsafe-inline`, no `unsafe-eval`.
- `style-src 'self' 'unsafe-inline'`. SvelteKit and the component library set
  style attributes, and a style can't run code.
- `frame-ancestors 'none'`, `base-uri 'none'`, `form-action 'self'`,
  `object-src 'none'`.

The hashes are computed from the embedded `index.html` when the server
starts, not written into the build or into a config file. They can't drift
from the build, and a script an attacker injects into the page isn't in that
file, so it doesn't run.

Why a header and not SvelteKit's `csp` option: for a prerendered build
SvelteKit emits a `<meta http-equiv>` tag, in which `frame-ancestors`,
`report-uri` and `sandbox` are ignored. The Go server can send the real header.

## Consequences

- **A malicious or compromised server is not stopped.** It controls the
  header and the page, so it can serve any script with a matching hash. This
  closes XSS and third-party scripts only. Nothing in the browser stops a
  server from serving different code until something like WAICT ships.
- A browser extension that reads the page is unaffected.
- A new inline script in the build is allowed automatically, because it is
  hashed with the rest. That is deliberate, since the build is trusted, but it
  means adding one needs a reviewer to notice. An external script from another
  origin is blocked, and so is `eval`.
- The hashes come from a regular expression over the build's `index.html`. A
  build that put a script in a form the expression misses would be blocked, and
  `e2e/csp.spec.ts` would fail on the violation.
- Trusted Types (`require-trusted-types-for 'script'`) is a possible second
  step. It isn't enabled, because nothing has yet checked that the generated
  code complies.
