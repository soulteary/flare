# Reverse proxies and support boundaries

> Scope: stable 0.6.8 and current source, with legacy 0.6.7 differences. Explicit
> trusted-proxy and canonical-origin settings ship in 0.6.8 and are absent in 0.6.7.

Use a dedicated hostname at the URL root, such as `https://flare.example.com/`.
Full subpath mounting at `/flare/` is unsupported: routes, forms, assets and
cookies contain root-relative paths. `FLARE_EXTERNAL_URL` does not add subpath support.
Keep the backend isolated from direct public access and do not cache private HTML,
account, editor or settings responses. Allow the documented 8 MiB import plus
multipart overhead in proxy limits and timeouts.

## Preserve Host with HTTPS termination

For both versions, preserve the public Host including any nondefault port and
set `FLARE_COOKIE_SECURE=true` when the browser uses HTTPS and the backend uses
HTTP. Clients must use HTTPS after forcing Secure cookies. Direct TLS is secure
automatically. A public HTTPS origin configured in current main also enforces
Secure; failures do not downgrade cookie policy.

Inside an existing TLS server for a dedicated hostname:

```nginx
location / {
    proxy_pass http://flare:5005;
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 10m;
}
```

```yaml
environment:
  FLARE_COOKIE_SECURE: "true"
```

Configure certificates and backend networking separately. Nginx Proxy Manager
uses the dedicated domain and HTTP upstream `flare:5005`; preserve Host and avoid
custom `/flare/` locations. See the deployment repository's
[Traefik example](https://github.com/soulteary/docker-flare/blob/main/example/traefik/docker-compose.yml).

In stable 0.6.7, forwarding headers do not establish cookie/source trust. Current
main also ignores them by default; ordinary Host-preserving proxies do not need
all advanced inputs below.

## Canonical origin and trusted proxies (current main)

- `FLARE_EXTERNAL_URL`: canonical HTTP(S) origin, for example
  `https://flare.example.com:8443`. No credentials, query or application path.
- `FLARE_ALLOWED_HOSTS`: comma-separated additional host/port authorities accepted
  as request aliases. They do not replace the canonical diagnostic origin.
- `FLARE_TRUSTED_PROXIES`: comma-separated IPs/CIDRs of actual trusted proxy peers.
  Only peers matching this list may supply supported `X-Forwarded-Proto`,
  `X-Forwarded-Host` and validated `X-Forwarded-For` information.

Use narrow addresses matching the proxy peer seen by Flare; do not copy a broad
network range merely to make validation pass. The final trusted proxy must
normalize supported headers and overwrite client-supplied authority/scheme values.
Multiple or comma-separated scheme/authority values are rejected. RFC `Forwarded`
is not supported on this trusted path; normalize/remove it at the last proxy.

If a trusted proxy rewrites Host to an internal name and sends no forwarded Host,
a configured canonical external origin supplies the authority. Otherwise the
effective origin must match the canonical origin or an allowed alias. Without a
canonical origin, an explicit allowed-host list still constrains authorities.
A configured HTTPS policy rejects a forwarded HTTP scheme instead of downgrading.
Source checks compare scheme, hostname and effective port.

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

In current main, `SOURCE_REJECTED` indicates source protection, `CSRF_INVALID`
indicates an invalid browser/session-bound token, and `AUTH_REQUIRED`,
`SESSION_EXPIRED` or `REAUTH_REQUIRED` indicate authentication requirements.
Some route-local failures are generic HTTP 403 rather than a structured code.
Use response status/code, request ID when supplied and terminal logs together.
An invalid configured/request authority may return `INVALID_INPUT` with HTTP 400.

Current development settings documents explicitly declare a `same-origin`
referrer policy. This preserves a non-opaque Origin on native form submissions
even when a proxy overrides the HTTP response policy with `no-referrer`.
Previously, that combination could produce `Origin: null` and `SOURCE_REJECTED`
on a legitimate settings save. Opaque and cross-origin requests remain rejected.
This does not correct rewritten Host headers or mismatched public scheme/port;
those still require the proxy configuration above.

Run `FLARE_BINARY=/absolute/path/to/flare node tests/settings/save.browser.cjs`
after generating resources and rebuilding to verify direct/proxied native saves,
proxy response-policy overrides, persisted reloads and hostile Origin rejection.

Correct the public origin/proxy configuration, reopen the page on that origin and
obtain fresh tokens. Never reuse tokens across sessions or put them in query URLs.
Use Compose mapping syntax (`FLARE_DISABLE_LOGIN: "true"`); list syntax such as
`- FLARE_DISABLE_LOGIN="true"` includes literal quotes and is invalid.

## Verify

1. Check the actual binary version, public URL, Host/port and Secure policy.
2. In a fresh browser, verify anonymous public/private navigation and management.
3. Sign in over public HTTPS, inspect host-only Secure/HttpOnly cookies, reload,
   submit settings and a disposable import, and sign out.
4. For current main, verify private category/entry filtering, recent-auth flows,
   aliases if configured, and rejection of untrusted forwarding headers.
5. Check dynamic placeholders and cache isolation separately. Root-page success
   alone does not validate private responses, writes or full proxy behavior.
