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

- [ ] 6.1 Implement the update endpoint (bearer-token gated) and verify an object's value changes while its id and used_by metadata are preserved
- [ ] 6.2 Verify update rejects requests that lack a valid bearer token

## 7. Server: Delete Endpoint (alrayyes/hush-hush#10)

- [ ] 7.1 Implement the delete endpoint (bearer-token gated) and verify a deleted object's subsequent get returns not-found
- [ ] 7.2 Verify delete rejects requests that lack a valid bearer token

## 8. Server: Audit Logging (alrayyes/hush-hush#11)

- [ ] 8.1 Implement audit log recording wired into create, read, update, and delete, and verify every one of the four produces an entry with object id, timestamp, and caller identity if presented
- [ ] 8.2 Implement audit log query support filtered by object id, caller identity, and time range, and verify each filter in isolation
- [ ] 8.3 Verify audit log entries cannot be modified or deleted once recorded

## 9. CLI: Inject Command (alrayyes/hush-hush#34)

- [ ] 9.1 Implement inject: seal a value to one or more configured recipient keys and call the create endpoint, and verify a new object appears on the server afterward
- [ ] 9.2 Verify inject runs correctly inside a CI job using only normal environment/config, with no CI-specific code path

## 10. CLI: Get Command (alrayyes/hush-hush#35)

- [ ] 10.1 Implement get: fetch and decrypt one value to stdout per call, and verify it prints the correct plaintext for an object it holds a matching key for
- [ ] 10.2 Verify get reports a clear decryption failure, not silent incorrect output, when the CLI holds no matching private key
- [ ] 10.3 Verify get runs correctly inside a CI job using only normal environment/config, with no CI-specific code path
- [ ] 10.4 Add Pact (pact-go) contract tests for create and get - a consumer test against the CLI's HTTP client producing a local pact file, and a provider verification test confirming the server satisfies it (alrayyes/hush-hush#28)

## 11. CLI: Update Command (alrayyes/hush-hush#13)

- [ ] 11.1 Implement update: seal a new value and call the update endpoint, and verify the object's server-side value changes accordingly
- [ ] 11.2 Add Pact contract tests for update, extending the same local pact file (alrayyes/hush-hush#29)

## 12. CLI: Delete Command (alrayyes/hush-hush#14)

- [ ] 12.1 Implement delete: call the delete endpoint, and verify the object is gone from the server afterward
- [ ] 12.2 Add Pact contract tests for delete, extending the same local pact file (alrayyes/hush-hush#30)

## 13. End-to-End Validation

- [ ] 13.1 Migrate one real secret from its prior source onto this service end to end, and verify its consuming pipeline still works after the switch
- [ ] 13.2 Remove the old copy of that secret and verify nothing still depends on it

## 14. Documentation

- [ ] 14.1 Write the README (requirements, install, usage, configuration) and verify a reader with no prior context can follow it to inject/get/update/delete a secret
- [ ] 14.2 Verify the README's examples contain no environment-specific assumptions
