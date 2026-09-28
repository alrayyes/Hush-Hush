# Spec Delta

## Purpose

Lets an operator see, at a glance from the Consumers page, how many
consumer read tokens each consumer holds, and jump straight to a filtered
view of just that consumer's tokens for management.

## ADDED Requirements

### Requirement: Consumers page shows a per-consumer token count

The Consumers page SHALL show, for each consumer, the count of consumer
read tokens issued to it.

#### Scenario: Consumer with tokens

- **WHEN** the Consumers page loads and a consumer has 3 consumer
  tokens
- **THEN** that consumer's row shows a "Tokens" value of `3`, rendered as a
  link to that consumer's filtered view

#### Scenario: Consumer with no tokens

- **WHEN** the Consumers page loads and a consumer has 0 consumer
  tokens
- **THEN** that consumer's row shows a "Tokens" value of `0`, rendered as
  plain text with no link

### Requirement: Consumer tokens can be filtered to a single consumer

The Settings page's Consumer tokens table SHALL support filtering to a
single named consumer via a URL query parameter, with a visible, removable
indicator of the active filter.

#### Scenario: Arriving with a consumer filter

- **WHEN** the Settings page loads with `?consumer=<name>` in the URL
- **THEN** the Consumer tokens table shows only tokens belonging to `<name>`,
  a removable "consumer: `<name>`" filter chip is shown, and the page is
  scrolled to the Consumer tokens section

#### Scenario: Clearing the consumer filter

- **WHEN** the consumer filter chip's remove control is activated
- **THEN** the Consumer tokens table shows every consumer's tokens again and
  the `consumer` query parameter is removed from the URL
