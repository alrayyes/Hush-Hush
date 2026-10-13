# Spec Delta

## ADDED Requirements

### Requirement: MCP endpoint mirrors the object operations

Hush-Hush SHALL expose an MCP (Model Context Protocol) endpoint at
`POST /mcp`, serving tools that mirror `hush-hush-cli`'s own operations:
`inject`, `get`, `update`, `delete`, `list`.

#### Scenario: An MCP client discovers the available tools

- **WHEN** an authenticated MCP client sends `tools/list`
- **THEN** the response includes `inject`, `get`, `update`, `delete`, and
  `list`

#### Scenario: A tool call performs the same operation its HTTP twin does

- **WHEN** an authenticated MCP client calls `inject` with an id and a
  sealed value
- **THEN** an object is created under that id, exactly as
  `POST /objects` would create it, and a create audit log entry is
  recorded

### Requirement: Every MCP tool call is authenticated

No MCP tool call SHALL succeed without a valid write bearer token or a
valid session - the same credential `requireWriteAccess` already checks
on every other write route, applied uniformly to all five tools.

#### Scenario: An unauthenticated request is rejected

- **WHEN** a `POST /mcp` request carries no valid bearer token or session
- **THEN** the server responds `401 Unauthorized` and no tool call runs

#### Scenario: A tool call is attributed to its verified actor

- **WHEN** an authenticated MCP client's tool call creates, updates, reads,
  or deletes an object
- **THEN** the resulting audit log entry's `actor_type`/`actor_id` reflect
  the same credential (`token` or `session`) that authenticated the
  request, exactly as the equivalent HTTP call would record
