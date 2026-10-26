# Weather refresh and failure behavior

> Availability: the homepage snapshot and retry behavior shipped in 0.6.5.
> The location picker, dependency 0.4.0 and migration behavior below belong to the
> unpublished 0.6.6 candidate.

The homepage keeps one in-memory weather snapshot. A missing or expired snapshot
starts a background fetch; page rendering never waits for that network request.
Concurrent visitors share a single in-flight fetch. The first page after startup
can therefore have no weather data; reload after the fetch completes to see it.
There is no browser polling or unsolicited background refresh when nobody visits.

An expired snapshot remains visible while the same city's weather is refreshed.
Failures retain that snapshot and wait 15 seconds before retrying on a later page
request. Consecutive failures increase the delay to 30, 60, 120, 240 and at most
300 seconds. A successful refresh resets the delay. Failed refreshes produce a
server warning rather than failing the homepage.

Changing the city does not display the old city's weather as if it belonged to
the new city. An in-flight old-city result is discarded after the change is
observed; a later page request starts the new-city fetch. Offline mode, disabled
weather and an empty location do not start homepage fetches.

The cache is process-local and is not a backup or shared cache. Existing upstream
coverage, timeouts and day/night calculation remain unchanged. The explicit
weather test in Settings still performs a direct test and is separate from the
homepage cache. Weather settings that cannot be written return HTTP 500, retain
the submitted fields and show a retryable error instead of implying success.

Deterministic tests use an injected clock and fetch function, including concurrent
cold reads, stale results, bounded retries, changed-city responses and failed
settings writes. They do not contact a live weather provider.

## 0.6.6 candidate: location selection and compatibility

Flare uses `funny-china-weather` v0.4.0 (Go 1.27.3 or later). Settings now
searches Tencent's location directory and requires selection of a specific city,
district or county. Candidates show province / city / county, including duplicate
names. Search is debounced, Chinese IME composition is allowed to finish before
searching, previous requests are canceled and late responses are ignored.
The four municipality names with a 市 suffix (including 北京市) are also accepted
in the search box and during save revalidation. Native buttons support keyboard selection. Empty/error states allow retry.
The server rechecks the candidate ID against the submitted search keyword before
saving; submitted display text cannot replace provider fields. Search keywords
are limited to 64 characters, with a three-second timeout, at most four concurrent
directory searches and a shared budget of 120 searches per minute.

Selected locations are persisted with exact provider fields, for example:

```yaml
ShowWeather: true
Location: "湖南, 常德, 津市"
WeatherLocation:
  ID: "<provider candidate ID>"
  Province: "湖南"
  City: "常德"
  County: "津市"
```

Use Settings to obtain the real candidate ID. `Location` remains the display and
legacy compatibility value. Homepage refreshes use the selected province/city/county
through `GetObservationAt`, without repeated directory lookups. Changing from a
county to a city explicitly clears the old county. Disabling weather preserves
the selected location. Settings tests display the address, temperature, humidity,
condition and upstream timestamp, or an explicit retryable failure message.

At startup, enabled legacy locations without `WeatherLocation` are searched once.
Only one matching candidate is migrated. Serialized provider addresses are searched
by their most specific field (county when present), then matched against their
exact province/city/county fields.
The four exact legacy municipality names 北京市、天津市、上海市、重庆市
use a compatibility mapping and migrate only after the provider confirms the
matching municipality province/city pair. Unavailable, missing or ambiguous results
leave the original configuration untouched; Settings asks the user to search and select.
Old concatenated province/city strings are not split or rewritten with guessed
suffixes. A failed disk write also leaves the previous configuration intact.
A manual selection made during migration takes precedence over the migration.

An empty location no longer triggers automatic startup IP selection. Settings
provides **Locate server outgoing IP** as a suggestion: this reflects the server's
proxy/VPN/egress, not the visitor's physical position. The user must confirm the
candidate and save it. Detection has a five-second deadline; no result is saved
by the detection endpoint. Search/detection/script routes require authentication
under the normal login policy and are absent in offline mode. The picker requires
JavaScript; the saved location remains visible when JavaScript is unavailable.

All v0.3.0 icon keys remain compatible. New fog/haze codes and intensity artwork
are handled by the dependency's selector; unknown codes use `unknown`.
Weather snapshots remain process-local and clear on restart.

The consumer compatibility test mocks provider responses and checks 44 codes,
day/night selection, parsed measurements, provider query fields and trusted SVG
output. Additional tests cover selected county persistence, duplicate-name choices,
forged IDs, old configuration migration, exact IP candidate matching, errors,
offline routes and authentication. They do not call live weather services.

Run the regular checks:

```sh
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...
```

Optional desktop/mobile browser checks require the Playwright installation used
by `tests/clock` and its Chromium browser (or set `CHROMIUM_EXECUTABLE` to a
compatible installed Chrome executable):

```sh
FLARE_WEATHER_BROWSER=1 go test ./internal/settings/weather -run TestWeatherBrowser -count=1 -v
```

These checks run the real embedded settings page against local provider fixtures,
covering duplicate candidates, county save/reload, missing/failed searches, late
responses, IP confirmation and horizontal overflow at desktop/mobile widths.
See the [v0.4.0 release notes](https://github.com/soulteary/funny-china-weather/releases/tag/v0.4.0)
for upstream location-input migration details.

Real-provider verification is separate from the offline suite and writes only to
temporary configuration directories:

```sh
go test -tags integration ./internal/settings/weather -run TestLiveWeatherSelectionAndMigration -count=1 -v
FLARE_WEATHER_LIVE_BROWSER=1 go test -tags integration ./internal/settings/weather -run TestLiveWeather -count=1 -v
```

The live checks search 北京、朝阳、津市, select and save every returned candidate,
query weather from the saved fields, and verify legacy migration. The optional
browser check repeats selection, save/reload and weather testing at desktop and
mobile widths. Set `CHROMIUM_EXECUTABLE` when using an installed Chrome instead of
Playwright's bundled browser. Live checks intentionally fail when upstream services
are unavailable; they are not part of ordinary CI.

A failed location search logs its original provider error on the server and
returns a localized category to Settings: invalid keyword (400), busy (429),
timeout (504), network connection failure (502), or other upstream error (502).
A network failure such as `connect: bad file descriptor` occurs before any
location result is received. It must not be interpreted as an unknown city.
The browser displays the server's diagnostic category rather than one generic
message for all failures.

The complete production router, authentication middleware and native listener
can also be checked against live providers, using a loopback-only listener and
temporary configuration:

```sh
go test -tags integration ./internal/server -run TestLiveNativeWeather -count=1 -v
```
