# Reverse proxies and support boundaries

> Scope: stable 0.6.15 and current source, with differences for stable 0.6.8–0.6.14 and legacy
> 0.6.7. Explicit trusted-proxy and canonical-origin settings ship in 0.6.8 and
> are absent in 0.6.7.

Use a dedicated hostname at the URL root, such as `https://flare.example.com/`.
Full subpath mounting at `/flare/` is unsupported: routes, forms, assets and
cookies contain root-relative paths. `FLARE_EXTERNAL_URL` does not add subpath support.
Keep the backend isolated from direct public access and do not cache private HTML,
account, editor or settings responses. Allow the documented 8 MiB import plus
multipart overhead in proxy limits and timeouts.

## HTTPS termination

Current source accepts writes behind a TLS-terminating proxy without additional
settings. Browsers attach `Sec-Fetch-Site` to requests for HTTPS (and loopback)
URLs. The browser computes it from the address the user visits, page scripts
cannot set it, and it does not depend on the scheme or Host that reach Flare.
A `same-origin` or `none` value is accepted; any other value is rejected. This
is the rule implemented by Go's `net/http.CrossOriginProtection`.

Requests without `Sec-Fetch-Site` come from browsers released before 2023, or
from any browser using plain HTTP on a non-loopback address. For these, the
`Origin` hostname and port (not the scheme) must match an authority Flare answers
to: the `Host` header it received, the authority supplied by a trusted proxy,
`FLARE_EXTERNAL_URL`, or a `FLARE_ALLOWED_HOSTS` entry. Preserve the public Host,
including any nondefault port, so that these browsers work as well. A request
with neither `Sec-Fetch-Site` nor `Origin`/`Referer` does not come from a browser
and passes this check; management writes still require the CSRF token.

Inside an existing TLS server for a dedicated hostname:

```nginx
location / {
    proxy_pass http://flare:5005;
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 10m;
}
```

Set `FLARE_COOKIE_SECURE=true` when every user reaches Flare over HTTPS, so that
cookies carry the `Secure` attribute:

```yaml
environment:
  FLARE_COOKIE_SECURE: "true"
```

With this setting, pages opened over plain HTTP can still be read but cannot sign
in or save: browsers do not return Secure cookies over HTTP, and Flare reports
`ORIGIN_SCHEME_MISMATCH`. Leave it unset when the same instance is also used
over HTTP, for example through a LAN address. Direct TLS is secure automatically.
An HTTPS `FLARE_EXTERNAL_URL` has the same effect; failures do not downgrade
cookie policy.

Configure certificates and backend networking separately. Nginx Proxy Manager
uses the dedicated domain and HTTP upstream `flare:5005`; preserve Host and avoid
custom `/flare/` locations. See the deployment repository's
[Traefik example](https://github.com/soulteary/docker-flare/blob/main/example/traefik/docker-compose.yml).

Stable 0.6.8 through 0.6.14 also compare the scheme, even when
`Sec-Fetch-Site` reports a same-origin request. Behind TLS termination they need
`FLARE_COOKIE_SECURE=true`, an HTTPS `FLARE_EXTERNAL_URL`, or a trusted proxy
that supplies `X-Forwarded-Proto`; otherwise every write fails with
`ORIGIN_SCHEME_MISMATCH`. In stable 0.6.7, forwarding headers do not establish
cookie/source trust and only the Host is compared. Forwarding headers are
ignored by default in every version.

## Canonical origin, allowed hosts and trusted proxies

- `FLARE_EXTERNAL_URL`: public HTTP(S) origin, for example
  `https://flare.example.com:8443`. No credentials, query or application path.
  It is used for diagnostics, its authority is accepted as a write source when a
  proxy rewrites Host, and an HTTPS value enforces Secure cookies. It does not
  restrict the Host values Flare answers to.
- `FLARE_ALLOWED_HOSTS`: comma-separated host/port authorities. When set, Flare
  answers only to these authorities and to the one in `FLARE_EXTERNAL_URL`; any
  other Host receives HTTP 400 with reason `HOST_NOT_ALLOWED` on every route.
- `FLARE_TRUSTED_PROXIES`: comma-separated IPs/CIDRs of actual trusted proxy peers.
  Only peers matching this list may supply supported `X-Forwarded-Proto`,
  `X-Forwarded-Host` and validated `X-Forwarded-For` information.

Without `FLARE_ALLOWED_HOSTS`, Flare answers to any Host that reaches it. A page
on an attacker-controlled name that resolves to the instance (DNS rebinding) is
then same-origin from the browser's point of view. It acts as an anonymous
visitor: it cannot use a signed-in session, because cookies belong to the real
hostname, but it can read whatever the instance shows without signing in, and
on an instance with login disabled it can also write. Set `FLARE_ALLOWED_HOSTS`
to the names and ports the instance is reached by when that matters.

Stable 0.6.8 through 0.6.14 treat `FLARE_EXTERNAL_URL` as a restriction too: a
request for any other Host, including a LAN address, receives HTTP 400. Current
source applies that restriction only through `FLARE_ALLOWED_HOSTS`. Add it when
upgrading an instance that relied on the previous behavior.

Use narrow addresses matching the proxy peer seen by Flare; do not copy a broad
network range merely to make validation pass. The final trusted proxy must
normalize supported headers and overwrite client-supplied authority/scheme values.
Multiple or comma-separated scheme/authority values are rejected. RFC `Forwarded`
is not supported on this trusted path; normalize/remove it at the last proxy.

If a trusted proxy rewrites Host to an internal name and sends no forwarded Host,
a configured external origin supplies the authority for the allowed-host check.
A configured HTTPS policy rejects a forwarded HTTP scheme instead of downgrading.
Without `Sec-Fetch-Site`, source checks compare hostname and effective port; the
scheme is compared only to reject an HTTP page under an explicit HTTPS policy.

These settings govern source/authentication policy. Dynamic bookmark placeholders
still use the request context described in [configuration](configuration.md#dynamic-bookmark-placeholders);
do not assume canonical-origin settings rewrite every generated bookmark URL.

## Write requests rejected even with login disabled

Login and write-source protection are independent. Preserve pending edits or
export a JSON draft before reloading a rejected form. Disabling login does not
permit cross-origin writes.

In stable 0.6.7, `write_source_rejected` identifies the global source failure;
0.6.6 can mislabel its plain-text 403 as an expired login. Preserve Host including
the port (`$http_host`, since `$host` may omit a nondefault port). Sending only
`X-Forwarded-Host` does not fix stable-version source validation.

Since 0.6.8, `SOURCE_REJECTED` indicates source protection, `CSRF_INVALID`
indicates an invalid browser/session-bound token, and `AUTH_REQUIRED`,
`SESSION_EXPIRED` or `REAUTH_REQUIRED` indicate authentication requirements.
Some route-local failures are generic HTTP 403 rather than a structured code.
Use response status/code, request ID when supplied and terminal logs together.
A request authority that cannot be accepted returns `INVALID_INPUT` with HTTP 400.

Rejection responses include a stable `error.reason`, a human-readable
`error.message`, an actionable `error.hint`, and a `request_id`. The editor shows
all four values while preserving the current draft. A rejected form submission is
a page navigation and receives an explanation page in the site language with the
same reason and request ID. Logs record the reason and request ID without copying
untrusted request header values. Include the reason and request ID in support reports.

| Status and code | Reason | Meaning |
| --- | --- | --- |
| 403 `SOURCE_REJECTED` | `FETCH_SITE_NOT_SAME_ORIGIN` | The browser reported a cross-site or same-site request |
| 403 `SOURCE_REJECTED` | `ORIGIN_HOST_MISMATCH`, `ORIGIN_PORT_MISMATCH` | No `Sec-Fetch-Site`, and the page hostname or port matches no authority Flare answers to; usually a proxy that rewrites Host or drops the public port |
| 403 `SOURCE_REJECTED` | `ORIGIN_SCHEME_MISMATCH` | No `Sec-Fetch-Site`, and an HTTP page under an explicit HTTPS policy |
| 403 `SOURCE_REJECTED` | `ORIGIN_OPAQUE`, `ORIGIN_INVALID`, `ORIGIN_AMBIGUOUS`, `FETCH_METADATA_AMBIGUOUS` | No `Sec-Fetch-Site` and an unusable `Origin`, or duplicated source headers |
| 400 `INVALID_INPUT` | `HOST_NOT_ALLOWED` | The Host is outside `FLARE_ALLOWED_HOSTS` |
| 400 `INVALID_INPUT` | `HOST_INVALID` | The Host, or the Host forwarded by a trusted proxy, is not a valid authority |
| 400 `INVALID_INPUT` | `FORWARDED_HEADER_UNSUPPORTED`, `FORWARDED_HEADER_AMBIGUOUS`, `FORWARDED_FOR_INVALID`, `FORWARDED_PROTO_INVALID`, `FORWARDED_PROTO_CONFLICT` | A peer in `FLARE_TRUSTED_PROXIES` sent forwarding headers Flare does not accept |

`Referer` is consulted only when `Origin` is absent; its reasons use the
`REFERER_` prefix. Stable 0.6.8 through 0.6.14 report every HTTP 400 case as
`REQUEST_AUTHORITY_INVALID`, and `ORIGIN_SCHEME_MISMATCH` there usually means
HTTPS termination without one of the settings listed above.

Homepage search and its suggestions only read. Current source does not apply the
source check to them, so searching works in every deployment; the allowed-host
restriction still applies. With `FLARE_VISIBILITY=PRIVATE` they remain behind
authentication and its token.

Settings documents explicitly declare a `same-origin` referrer policy. This
preserves a non-opaque Origin on native form submissions even when a proxy
overrides the HTTP response policy with `no-referrer`. Without it, browsers that
send no `Sec-Fetch-Site` would submit `Origin: null` and be rejected. Opaque and
cross-origin requests from such browsers remain rejected.

Run `FLARE_BINARY=/absolute/path/to/flare node tests/settings/save.browser.cjs`
after generating resources and rebuilding to verify direct/proxied native saves,
proxy response-policy overrides, persisted reloads and hostile Origin rejection.
`tests/editor/write-source.browser.cjs` covers proxies that rewrite Host, drop
the public port or send only forwarding headers, with and without `Sec-Fetch-Site`.

Reopen the page on the corrected origin to obtain fresh tokens. Never reuse
tokens across sessions or put them in query URLs.
Use Compose mapping syntax (`FLARE_DISABLE_LOGIN: "true"`); list syntax such as
`- FLARE_DISABLE_LOGIN="true"` includes literal quotes, which releases through
0.6.14 reject as invalid. Current source ignores one pair of surrounding quotes.

## Verify

1. Check the actual binary version, public URL, Host/port and Secure policy.
2. In a fresh browser, verify anonymous public/private navigation and management.
3. Sign in over public HTTPS, inspect host-only HttpOnly cookies (Secure when
   the HTTPS policy is set), reload, submit settings and a disposable import,
   and sign out.
4. Verify private category/entry filtering, recent-auth flows, aliases if
   configured, and rejection of untrusted forwarding headers.
5. Check dynamic placeholders and cache isolation separately. Root-page success
   alone does not validate private responses, writes or full proxy behavior.
