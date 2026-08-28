## Why

Secrets currently reach CI pipelines and deploy scripts through a mix of a git+sops workflow that needs a private key on a workstation for every edit, and hand-set platform-native CI secrets with no central record of what depends on what. A lightweight, standalone secrets object store removes the private key from the write path entirely, gives every consumer one consistent way to fetch and decrypt a value, and keeps a queryable record of what consumes what - while staying generic enough to run anywhere, not just one environment.

## What Changes

- New standalone Go service exposing CRUD over sealed secret objects: each object is encrypted (age) to whichever public key(s) are chosen at write time, multi-recipient when a secret is genuinely shared across several consumers.
- New CLI (`inject`, `get`, `update`, `delete`) implementing the same operations end to end - the writer's only interface to the service, and the same binary every consumer (CI job, deploy script) runs to fetch and decrypt a value locally. The service itself never computes or returns plaintext.
- Every create/read/update/delete call is recorded to an audit log (object id, timestamp, caller identity if presented), queryable by object, caller, and time range.
- v1 read path is flat: no pre-fetch authorization check. The audit log is what answers "did anyone fetch this" during a rotation or incident - it is a deliberate v1 scope decision, not an oversight, with per-consumer read authorization deferred to a later change.
- v1 write path is gated by a single bearer token, checked on create/update/delete.
- SQLite-backed storage (objects, used_by lineage, audit log) - chosen because lineage queries, audit queries, and shared-secret tracking are all relational.
- `used_by` lineage (which repo or host consumes which secret) is stored and queryable, not just implied by convention.

## Capabilities

### New Capabilities

- `secret-objects`: CRUD lifecycle for sealed secret objects - create, read, update, delete, `used_by` lineage tracking, and the bearer-token write-path auth model.
- `audit-log`: per-action audit trail for every create/read/update/delete call, queryable by object, caller, and time range.
- `cli`: command-line client (`inject`/`get`/`update`/`delete`) implementing the service's API, usable directly inside CI jobs with no bespoke wrapper.

### Modified Capabilities

None - this is a new, standalone service with no pre-existing specs in this repo.

## Impact

- New repository content: a Go HTTP service and a Go CLI, both built against one OpenAPI contract.
- New dependencies: `age` encryption (e.g. `filippo.io/age`), SQLite via `modernc.org/sqlite` (pure Go, no cgo, to keep single-binary cross-compilation trivial).
- No plaintext ever transits or persists on the service; only sealed ciphertext.
- Downstream: every repo currently consuming a secret through the prior ad hoc means will need to migrate to fetching from this service - tracked as separate migration work per consuming repo, not part of this change's implementation.
