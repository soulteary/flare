# Reverse proxies and support boundaries

Use a dedicated host at the URL root, for example `https://flare.example.com/`.
Full subpath mounting such as `/flare/` is not currently supported: routes,
forms, asset URLs and cookies include root-relative paths. Rewriting only `/` or
adding a proxy prefix does not implement complete subpath support. Serve at a
separate host rather than claiming `/flare/` works after a partial rewrite.

When the proxy terminates HTTPS and uses HTTP to the application:

- Configure `FLARE_COOKIE_SECURE=true` and keep accessing the public service via HTTPS.
- Preserve the original `Host`, including a nondefault public port. Do not blindly
  trust client-supplied forwarding headers or expose the backend publicly.
- Forwarding `X-Forwarded-Proto` does not change Flare's cookie trust policy.
- Ensure upload body limits and timeouts allow the documented 8 MiB bookmark import.
- Do not cache private HTML, editor documents, or settings responses at the proxy.

## Nginx

Inside an existing TLS server for the dedicated hostname:

```nginx
location / {
    proxy_pass http://flare:5005;
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 10m;
}
```

Configure TLS/certificates and backend network access separately. For Nginx Proxy
Manager use the dedicated domain, HTTP upstream `flare:5005`, and preserve Host;
do not configure a custom `/flare/` location. See the deployment repository's
[Traefik example](https://github.com/soulteary/docker-flare/blob/main/example/traefik/docker-compose.yml)
and adapt its public hostname, TLS resolver and network to your installation.

## Write requests rejected even with login disabled

Login and write-source protection are independent. Disabling login does not allow
cross-origin writes. If settings or editor saves return HTTP 403 with code
`write_source_rejected`, compare the browser's `Origin`/`Referer` host and port
with the `Host` received by Flare. The proxy must preserve the public host and
port, rather than replace them with an upstream name such as `flare:5005`.
Sending only `X-Forwarded-Host` does not fix this: Flare does not trust it.
For Nginx, use `proxy_set_header Host $http_host;` as above; `$host` can drop a
nondefault public port. Direct Docker port publishing alone preserves Host.

In 0.6.6, the editor can mislabel this plain-text 403 as an expired login.
Changing credentials or enabling/disabling login does not resolve a Host mismatch.
Preserve/export pending edits before reloading, correct the proxy configuration,
then retry. Set the environment variable using Compose's mapping syntax:

```yaml
environment:
  FLARE_DISABLE_LOGIN: "true"
```

The list item `- FLARE_DISABLE_LOGIN="true"` includes literal quotes in the
environment value and triggers an invalid-boolean warning.

## Verify

1. In a fresh browser confirm private routes redirect to login when configured.
2. Sign in over public HTTPS, inspect the Secure/HttpOnly session cookie, reload,
   open the editor, and sign out.
3. Submit search settings and a small disposable import; check protection errors
   do not come from a lost Host/port or a cached pre-restart form.
4. Verify dynamic bookmark placeholders against the public URL. The backend's
   direct HTTP scheme is distinct from the external TLS scheme; forwarded headers
   are not a general trusted-proxy configuration.
