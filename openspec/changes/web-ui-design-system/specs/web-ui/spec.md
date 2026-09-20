# Spec Delta

## Purpose

Gives every page - authenticated or not - the same visual chrome, a
mobile-first responsive layout, a theme choice, properly rendered
Markdown content, and installability, instead of each page inventing its
own ad hoc styling.

## ADDED Requirements

### Requirement: Consistent site chrome on every page

Every page SHALL render inside the same header/nav and footer chrome.
When a valid session exists, the header SHALL show the authenticated
navigation links (Secrets, Audit log, Settings, Log out); when it does
not, the header SHALL show no authenticated-only link.

#### Scenario: Authenticated visitor keeps the nav on a static page

- **WHEN** an authenticated admin navigates from the secrets overview to
  the changelog, disclaimer, or privacy page
- **THEN** the same header nav (Secrets, Audit log, Settings, Log out)
  remains visible on that page

#### Scenario: Unauthenticated visitor sees no broken nav links

- **WHEN** a visitor with no valid session opens the changelog,
  disclaimer, privacy, or login page
- **THEN** the header shows no link to Secrets, Audit log, or Settings

### Requirement: Layout is mobile-first and responsive

Every page's base styles SHALL target a phone-width viewport, with wider
layouts applied through `min-width` media queries, and no page SHALL
require horizontal scrolling at a 320px viewport width.

#### Scenario: Narrow viewport needs no horizontal scroll

- **WHEN** any page is rendered at a 320px viewport width
- **THEN** the page's content fits without horizontal scrolling

#### Scenario: Wide viewport gets the expanded layout

- **WHEN** any page is rendered at a desktop viewport width
- **THEN** the page uses its wider layout rather than the phone-width base
  styles

### Requirement: Theme preference

The web UI SHALL offer a light/dark theme toggle. A visitor's explicit
choice SHALL persist across visits; absent an explicit choice, the theme
SHALL follow the browser/OS `prefers-color-scheme` setting.

#### Scenario: Explicit choice persists

- **WHEN** a visitor selects a theme and returns later in the same
  browser
- **THEN** the previously selected theme is applied, not the OS default

#### Scenario: No explicit choice follows the OS setting

- **WHEN** a visitor with no stored theme preference opens the site
- **THEN** the applied theme matches their browser/OS
  `prefers-color-scheme` setting

### Requirement: Changelog renders as formatted content

The changelog page SHALL render `CHANGELOG.md`'s content as formatted
HTML (headings, lists, links) rather than as preformatted plain text,
while still reflecting the file's real content unchanged.

#### Scenario: Changelog headings and lists render as markup

- **WHEN** the changelog page is opened
- **THEN** `CHANGELOG.md`'s headings and list items render as HTML
  headings and lists, not as literal `#`/`-` characters in a text block

### Requirement: Installable as a progressive web app

The web UI SHALL be installable as a PWA: it SHALL serve a web app
manifest and register a service worker that lets the app be added to a
device's home screen and launched as a standalone app.

#### Scenario: Manifest and service worker are served

- **WHEN** the web UI is loaded in a browser that supports installable
  PWAs
- **THEN** a valid web app manifest is linked from the page and a service
  worker registers successfully
