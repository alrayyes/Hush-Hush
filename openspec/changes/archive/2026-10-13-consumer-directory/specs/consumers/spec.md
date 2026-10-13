# Spec Delta

## ADDED Requirements

### Requirement: Consumer listing is paginated and filterable

The consumer listing endpoint SHALL accept an optional page number, an
optional page size, and an optional name filter, returning only consumers
whose name contains the filter text (case-insensitive) when one is given,
and SHALL report the total number of matching consumers so page-number
navigation can be rendered. Omitting pagination parameters SHALL return
the complete, unpaginated listing (`consumer-combobox`'s existing
behaviour).

#### Scenario: Filtering by name

- **WHEN** the consumer listing is requested with a name filter
- **THEN** only consumers whose name contains that filter text are
  returned

#### Scenario: Paginated request returns one page and a total count

- **WHEN** the consumer listing is requested with a page number and page
  size
- **THEN** the response contains at most that many consumers, matching
  that page, and the total count of all matching consumers

#### Scenario: No pagination parameters returns everything

- **WHEN** the consumer listing is requested with no page, page size, or
  filter parameters
- **THEN** every distinct consumer is returned, matching the behaviour
  before this requirement existed

### Requirement: Consumer listing includes a secret count

Each consumer in the listing SHALL include the number of stored secret
objects whose `used_by` includes it.

#### Scenario: Count reflects how many secrets reference a consumer

- **WHEN** the consumer listing is requested
- **THEN** each consumer's entry shows the number of secret objects whose
  `used_by` list includes that consumer's name

### Requirement: Consumer directory page

The authenticated app SHALL provide a consumers page listing every
consumer with its secret count, paginated with page-number navigation and
filterable by name, and selecting a consumer SHALL navigate to the
secrets overview filtered to that consumer.

#### Scenario: Selecting a consumer filters the secrets overview

- **WHEN** the admin selects a consumer from the directory page
- **THEN** the secrets overview opens showing only secrets whose
  `used_by` includes that consumer

#### Scenario: Filtering the directory narrows the list

- **WHEN** the admin enters a name filter on the directory page
- **THEN** only matching consumers are shown, with page-number navigation
  reflecting the filtered total
