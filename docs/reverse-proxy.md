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

## Verify

1. In a fresh browser confirm private routes redirect to login when configured.
2. Sign in over public HTTPS, inspect the Secure/HttpOnly session cookie, reload,
   open the editor, and sign out.
3. Submit search settings and a small disposable import; check protection errors
   do not come from a lost Host/port or a cached pre-restart form.
4. Verify dynamic bookmark placeholders against the public URL. The backend's
   direct HTTP scheme is distinct from the external TLS scheme; forwarded headers
   are not a general trusted-proxy configuration.
