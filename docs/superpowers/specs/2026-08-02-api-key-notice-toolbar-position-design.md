# API Key Notice Toolbar Position Design

## Goal

Move the configured API key notice directly beside the Status filter on the API key management page without changing the notice content, settings, or the placement of existing toolbar actions.

## Layout

Add an optional `afterFilters` slot to the shared `DataTableToolbar`. Render it immediately after the faceted filter chips in both toolbar layout branches.

The API key toolbar order becomes:

1. Name search
2. API key search
3. Status filter
4. API key notice
5. Existing right-aligned actions

The notice keeps its current bounded width and compact alert styling. The toolbar remains a wrapping flex layout, so narrow screens may place the notice on the next line without overlapping or shrinking the filter controls.

## Compatibility

`afterFilters` is optional, so all existing table toolbars retain their current behavior. The existing `preActions` slot remains in the right-aligned action cluster and is not repurposed.

The API key notice setting, status payload field, text rendering, and translations remain unchanged.

## Verification

- Add a toolbar component test that verifies `afterFilters` renders after the filter chips and before `preActions`.
- Run the focused toolbar and API key notice tests.
- Run frontend type checking, focused lint, formatting checks, and a production build.
- Inspect the API key page at desktop and mobile widths to confirm the notice follows Status and wraps cleanly.
