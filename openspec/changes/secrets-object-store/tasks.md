## 1. OpenAPI Spec

- [x] 1.1 Write the OpenAPI spec covering create, read, update, and delete endpoints plus audit-log query, and verify it lints clean with Redocly
- [x] 1.2 Get the spec reviewed and merged as the agreed contract before any server or CLI implementation starts

## 2. Server: Storage Schema

- [x] 2.1 Define the SQLite schema for objects, used_by, and the audit log, and verify migrations apply cleanly to a fresh database

## 3. Server: Create Endpoint (alrayyes/hush-hush#31)

- [x] 3.1 Implement the create endpoint (bearer-token gated) and verify a sealed object round-trips through storage unchanged
- [x] 3.2 Verify create rejects requests that lack a valid bearer token

## 4. Server: Get Endpoint (alrayyes/hush-hush#32)

- [x] 4.1 Implement the get endpoint (unauthenticated) and verify it returns stored ciphertext exactly as sealed, and returns not-found for an unknown id

## 5. Server: used_by Lineage (alrayyes/hush-hush#33)

- [x] 5.1 Implement used_by storage and query support and verify a stored object's used_by list is retrievable

## 6. Server: Update Endpoint (alrayyes/hush-hush#9)

- [x] 6.1 Implement the update endpoint (bearer-token gated) and verify an object's value changes while its id and used_by metadata are preserved
- [x] 6.2 Verify update rejects requests that lack a valid bearer token

## 7. Server: Delete Endpoint (alrayyes/hush-hush#10)

- [x] 7.1 Implement the delete endpoint (bearer-token gated) and verify a deleted object's subsequent get returns not-found
- [x] 7.2 Verify delete rejects requests that lack a valid bearer token

## 8. Server: Audit Log Recording (alrayyes/hush-hush#58)

- [x] 8.1 Implement audit log recording wired into create, read, update, and delete, and verify every one of the four produces an entry with object id, timestamp, and the X-Caller header's value if presented
- [x] 8.2 Verify audit log entries cannot be modified or deleted once recorded

## 9. Server: Audit Log Query Endpoint (alrayyes/hush-hush#59)

- [x] 9.1 Implement audit log query support filtered by object id, caller identity, and time range, and verify each filter in isolation

## 10. CLI: HTTP Client & Contract Tests (alrayyes/hush-hush#27)

Built before the CLI's own commands, not after - the client is what a Pact
consumer test exercises, and the CLI's cobra commands are a thin layer of
key management and flag parsing on top of it, per
[`design.md`](openspec/changes/secrets-object-store/design.md).

- [x] 10.1 Implement an HTTP client (create, get, update, delete) for internal/client, the transport the CLI's own commands will wrap
- [x] 10.2 Add Pact (pact-go) contract tests for all four actions - consumer tests against the client producing a local pact file, and a provider verification test confirming the server satisfies it

## 11. CLI: Inject Command (alrayyes/hush-hush#34)

- [x] 11.1 Implement inject: seal a value to one or more configured recipient keys and call the create endpoint, and verify a new object appears on the server afterward
- [x] 11.2 Verify inject runs correctly inside a CI job using only normal environment/config, with no CI-specific code path

## 12. CLI: Get Command (alrayyes/hush-hush#35)

- [x] 12.1 Implement get: fetch and decrypt one value to stdout per call, and verify it prints the correct plaintext for an object it holds a matching key for
- [x] 12.2 Verify get reports a clear decryption failure, not silent incorrect output, when the CLI holds no matching private key
- [x] 12.3 Verify get runs correctly inside a CI job using only normal environment/config, with no CI-specific code path

## 13. CLI: Update Command (alrayyes/hush-hush#13)

- [x] 13.1 Implement update: seal a new value and call the update endpoint, and verify the object's server-side value changes accordingly

## 14. CLI: Delete Command (alrayyes/hush-hush#14)

- [x] 14.1 Implement delete: call the delete endpoint, and verify the object is gone from the server afterward

## 15. End-to-End Validation

- [ ] 15.1 Migrate one real secret from its prior source onto this service end to end, and verify its consuming pipeline still works after the switch
- [ ] 15.2 Remove the old copy of that secret and verify nothing still depends on it

## 16. Documentation

- [ ] 16.1 Write the README (requirements, install, usage, configuration) and verify a reader with no prior context can follow it to inject/get/update/delete a secret
- [ ] 16.2 Verify the README's examples contain no environment-specific assumptions
