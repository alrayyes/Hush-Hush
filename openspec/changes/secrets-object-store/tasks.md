## 1. OpenAPI Spec

- [ ] 1.1 Write the OpenAPI spec covering create, read, update, and delete endpoints plus audit-log query, and verify it lints clean with Redocly
- [ ] 1.2 Get the spec reviewed and merged as the agreed contract before any server or CLI implementation starts

## 2. Server: Storage Foundation

- [ ] 2.1 Define the SQLite schema for objects, used_by, and the audit log, and verify migrations apply cleanly to a fresh database
- [ ] 2.2 Implement the create endpoint (bearer-token gated) and verify a sealed object round-trips through storage unchanged
- [ ] 2.3 Implement the get endpoint (unauthenticated) and verify it returns stored ciphertext exactly as sealed, and returns not-found for an unknown id
- [ ] 2.4 Implement used_by storage and query support and verify a stored object's used_by list is retrievable

## 3. Server: Update & Delete

- [ ] 3.1 Implement the update endpoint (bearer-token gated) and verify an object's value changes while its id and used_by metadata are preserved
- [ ] 3.2 Implement the delete endpoint (bearer-token gated) and verify a deleted object's subsequent get returns not-found
- [ ] 3.3 Verify create, update, and delete each reject requests that lack a valid bearer token

## 4. Server: Audit Logging

- [ ] 4.1 Implement audit log recording wired into create, read, update, and delete, and verify every one of the four produces an entry with object id, timestamp, and caller identity if presented
- [ ] 4.2 Implement audit log query support filtered by object id, caller identity, and time range, and verify each filter in isolation
- [ ] 4.3 Verify audit log entries cannot be modified or deleted once recorded

## 5. CLI: inject & get

- [ ] 5.1 Implement inject: seal a value to configured recipient key(s) and call the create endpoint, and verify a new object appears on the server afterward
- [ ] 5.2 Implement get: fetch and decrypt one value to stdout per call, and verify it prints the correct plaintext for an object it holds a matching key for
- [ ] 5.3 Verify get reports a clear decryption failure, not silent incorrect output, when the CLI holds no matching private key
- [ ] 5.4 Verify get and inject both run correctly inside a CI job using only normal environment/config, with no CI-specific code path

## 6. CLI: update & delete

- [ ] 6.1 Implement update: seal a new value and call the update endpoint, and verify the object's server-side value changes accordingly
- [ ] 6.2 Implement delete: call the delete endpoint, and verify the object is gone from the server afterward

## 7. End-to-End Validation

- [ ] 7.1 Migrate one real secret from its prior source onto this service end to end, and verify its consuming pipeline still works after the switch
- [ ] 7.2 Remove the old copy of that secret and verify nothing still depends on it

## 8. Documentation

- [ ] 8.1 Write the README (requirements, install, usage, configuration) and verify a reader with no prior context can follow it to inject/get/update/delete a secret
- [ ] 8.2 Verify the README's examples contain no environment-specific assumptions
