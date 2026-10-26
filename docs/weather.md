# Weather refresh and failure behavior

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

## Weather dependency compatibility

Flare uses `funny-china-weather` v0.3.0. This release renames embedded icon keys;
Flare obtains those keys from the library's weather-code selector and passes them
to `GetIconSVG` through an embedded-asset-only template adapter, so no
application configuration migration is needed.
Unknown codes now display the library's neutral `unknown` artwork. Weather
snapshots are in memory only and are cleared when the upgraded process starts.

The consumer compatibility test mocks the provider response and checks 38 codes,
day/night resource selection, parsed measurements and trusted inline SVG output.
It does not contact the external weather service or assert browser animation
behavior. The upstream naming changes are documented in the
[v0.3.0 release notes](https://github.com/soulteary/funny-china-weather/releases/tag/v0.3.0).
