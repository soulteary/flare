# Go Report Card

**Grade: A+** (98.5%)

| Metric | Value |
| ------ | ----- |
| Files | 185 |
| Issues | 32 |

## Checks

| Check | Score |
| ----- | ----- |
| go_vet | 100% |
| gofmt | 100% |
| ineffassign | 100% |
| gocyclo | 87% |
| license | 100% |
| misspell | 95% |

## Issues

### gocyclo

- `config/data/navigation_index.go`
  - Line 43: cyclomatic complexity 18 for function SearchNavigationMatches
- `internal/auth/formvalue_test.go`
  - Line 17: cyclomatic complexity 17 for function TestLoginFormValuePrecedence
- `internal/logger/request_handler_test.go`
  - Line 134: cyclomatic complexity 36 for function TestNewFiberWithConfig_RequestOutcomes
- `internal/settings/search/search.go`
  - Line 24: cyclomatic complexity 33 for function updateSearchOptions
- `internal/pages/home/bookmark.go`
  - Line 19: cyclomatic complexity 23 for function GenerateBookmarkNavigationTemplate
- `config/data/search_test.go`
  - Line 10: cyclomatic complexity 21 for function TestSearchSettingsPreserveOtherSettingsAndRejectInvalidChanges
- `config/data/clock_test.go`
  - Line 9: cyclomatic complexity 20 for function TestClockSettingsPersistenceAndValidation
- `internal/formvalue/formvalue.go`
  - Line 16: cyclomatic complexity 16 for function Get
- `internal/bookmarks/transfer.go`
  - Line 175: cyclomatic complexity 40 for function Build
  - Line 62: cyclomatic complexity 29 for function parseHTML
  - Line 146: cyclomatic complexity 22 for function URLKey
  - Line 19: cyclomatic complexity 16 for function Parse
- `config/data/editor_document.go`
  - Line 156: cyclomatic complexity 33 for function commitEditorDocument
  - Line 110: cyclomatic complexity 16 for function ValidateEditorDocument
- `internal/logger/request_handler.go`
  - Line 52: cyclomatic complexity 25 for function NewFiberWithConfig
- `internal/server/request_source.go`
  - Line 16: cyclomatic complexity 24 for function protectWriteSource
- `internal/search/search.go`
  - Line 119: cyclomatic complexity 22 for function parseTemplate
  - Line 51: cyclomatic complexity 20 for function Validate
  - Line 179: cyclomatic complexity 19 for function Resolve
- `internal/resources/mdi/mdi_test.go`
  - Line 89: cyclomatic complexity 22 for function TestIconRoute_ServesSVGAndHTTPRequests
- `config/data/settings_concurrency_test.go`
  - Line 13: cyclomatic complexity 19 for function TestConcurrentSettingsUpdatesPreserveIndependentFields
- `config/data/parsed_cache_test.go`
  - Line 132: cyclomatic complexity 18 for function TestIndexedSearchScopeOrderAndInvalidation
  - Line 12: cyclomatic complexity 17 for function TestParsedSnapshotsDoNotAliasReturnedModels
- `internal/settings/appearance/appearance.go`
  - Line 25: cyclomatic complexity 34 for function updateAppearanceOptions
- `internal/pages/home/weather_cache_test.go`
  - Line 173: cyclomatic complexity 23 for function TestWeatherWidgetWaitsForSnapshotOnHomeAndHelp
- `internal/pages/home/home.go`
  - Line 262: cyclomatic complexity 21 for function renderSearch
- `internal/pages/editor/transfer_test.go`
  - Line 22: cyclomatic complexity 21 for function TestTransferHTTP
- `internal/server/navigation_html_test.go`
  - Line 16: cyclomatic complexity 16 for function TestNavigationFieldsAreTextAndEscapedAttributes
- `tools/baseline/main.go`
  - Line 293: cyclomatic complexity 25 for function main
  - Line 155: cyclomatic complexity 22 for function runScenario
- `internal/settings/search/security.go`
  - Line 73: cyclomatic complexity 23 for function validRequestSource

### misspell

- `embed/assets/css/home/bookmarks.css`
  - Line 1: "bookmakrs" is a misspelling of "bookmarks"
  - Line 5: "bookmakrs" is a misspelling of "bookmarks"
  - Line 12: "bookmakrs" is a misspelling of "bookmarks"
  - Line 17: "bookmakrs" is a misspelling of "bookmarks"
  - Line 23: "bookmakrs" is a misspelling of "bookmarks"
  - Line 28: "bookmakrs" is a misspelling of "bookmarks"
  - Line 39: "bookmakrs" is a misspelling of "bookmarks"
  - Line 45: "bookmakrs" is a misspelling of "bookmarks"
  - Line 52: "bookmakrs" is a misspelling of "bookmarks"
  - Line 53: "bookmakrs" is a misspelling of "bookmarks"
  - Line 58: "bookmakrs" is a misspelling of "bookmarks"
  - Line 62: "bookmakrs" is a misspelling of "bookmarks"
  - Line 67: "bookmakrs" is a misspelling of "bookmarks"
  - Line 73: "bookmakrs" is a misspelling of "bookmarks"
  - Line 74: "bookmakrs" is a misspelling of "bookmarks"
  - Line 75: "bookmakrs" is a misspelling of "bookmarks"
  - Line 76: "bookmakrs" is a misspelling of "bookmarks"
  - Line 77: "bookmakrs" is a misspelling of "bookmarks"
  - Line 82: "bookmakrs" is a misspelling of "bookmarks"
  - Line 83: "bookmakrs" is a misspelling of "bookmarks"
  - Line 89: "bookmakrs" is a misspelling of "bookmarks"
  - Line 90: "bookmakrs" is a misspelling of "bookmarks"
  - Line 101: "bookmakrs" is a misspelling of "bookmarks"
  - Line 103: "bookmakrs" is a misspelling of "bookmarks"
- `embed/templates/home.html`
  - Line 101: "bookmakrs" is a misspelling of "bookmarks"
- `internal/resources/templates/html/home.html`
  - Line 18: "bookmakrs" is a misspelling of "bookmarks"
- `internal/resources/mdi/mdi-cheat-sheets/css/materialdesignicons.min.css`
  - Line 1: "stange" is a misspelling of "strange"
- `tests/mobile/desktop-columns.browser.cjs`
  - Line 12: "bookmakrs" is a misspelling of "bookmarks"
- `embed/assets/css/base.css`
  - Line 91: "bookmakrs" is a misspelling of "bookmarks"
- `internal/resources/mdi/mdi-cheat-sheets/index.html`
  - Line 476: "stange" is a misspelling of "strange"
- `config/define/style.go`
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
  - Line 3: "bookmakrs" is a misspelling of "bookmarks"
- `embed/assets/css/home/apps.css`
  - Line 96: "bookmakrs" is a misspelling of "bookmarks"
  - Line 98: "bookmakrs" is a misspelling of "bookmarks"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action)._
