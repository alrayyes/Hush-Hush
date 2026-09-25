# Spec Delta

## ADDED Requirements

### Requirement: WebMCP tools for read-only object metadata

The embedded web UI SHALL declare WebMCP tools for listing stored objects
and inspecting one object's metadata, for an in-browser agent, wherever
the browser supports the WebMCP API.

#### Scenario: A WebMCP-aware agent lists stored objects

- **WHEN** an in-browser agent calls the `list_objects` tool on an
  authenticated page
- **THEN** it receives the same object metadata (id, description,
  used_by) the page's own secrets overview already shows

#### Scenario: A WebMCP-aware agent inspects one object's metadata

- **WHEN** an in-browser agent calls the `get_object_metadata` tool with
  a known id
- **THEN** it receives that object's metadata, or a clear error if no
  object exists under that id

#### Scenario: The browser doesn't support WebMCP

- **WHEN** the page loads in a browser without `document.modelContext`
- **THEN** tool registration is a silent no-op - no error, no broken
  page load

### Requirement: No secret value or destructive operation is ever exposed

No WebMCP tool SHALL return a secret's sealed value, and no WebMCP tool
SHALL perform a create, update, or delete operation.

#### Scenario: Tool output never includes a sealed value

- **WHEN** any WebMCP tool registered by this page returns a result
- **THEN** that result contains only metadata (id, description,
  used_by) - never the object's ciphertext
