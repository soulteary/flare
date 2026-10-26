# Choosing built-in icons

> Availability: included in Flare 0.6.5 and later; not included in 0.6.4.

In an entry form, open **More options**, then **Choose icon** beside the icon
input. Search uses English substrings of the existing Material Design icon names,
ignores letter case, and displays at most 48 results per page. Selecting a result
changes only the icon field. Cancel or Escape keeps the current value and all
pending entry fields. Complete the entry form and use **Save changes** to persist.

The picker only browses the bundled icon names. Existing manually entered icon
values and image URLs continue using the existing preview and rendering rules.
It does not add uploads or a new icon library. The search endpoint and preview
endpoint require the same authentication as the editor. Results contain names,
not SVG documents; previews are fetched on demand.

Run `go test ./internal/resources/mdi ./internal/pages/editor -count=1` for search,
pagination, bounds and authentication checks. Run
`node tests/editor/icon-picker.browser.cjs` with the Playwright dependency installed
under `tests/clock`; `CHROMIUM_EXECUTABLE` can select an existing browser. The
browser suite uses mocked HTTP responses and checks integration with the actual
entry form, field preservation, selection/cancel, pagination, retry and width.
