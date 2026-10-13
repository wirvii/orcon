---
title: TLS and reverse proxy
description: Serve Orcon over HTTPS, either with its own certificate or behind a proxy, and set the public address.
sources:
  - ADR-0083
  - ADR-0038
  - ADR-0039
  - ADR-0052
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# TLS and reverse proxy

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the configuration schema removes this note.

By default Orcon listens only on the loopback address, `127.0.0.1:8080`, and you reach it from the same machine. To use it from other machines you must serve it over HTTPS. This page explains why, and the two ways to do it.

## Why HTTPS is required

Orcon's session cookie is `__Host-orcon_session`, marked `Secure` and `HttpOnly`. A browser sends such a cookie only over HTTPS, or on `localhost`. So sign-in over plain HTTP works only on the loopback address. The same reason keeps the first-run wizard from offering the recovery kit anywhere else: the kit must not travel unencrypted. Passkeys also depend on the address: the relying party identifier is the host of your public address.

## Set the public address

`publicUrl` (environment variable `ORCON_PUBLIC_URL`) is the address that your users type. It defaults to `http://127.0.0.1:8080`. Set it to the real address, for example `https://orcon.example.com`.

`orcon config check` reports an error when `publicUrl` starts with `http://`, its host is not a loopback address, and no TLS is configured. The error asks you to set `http.tls.*`, or to terminate TLS in a proxy and use `https://`.

## Option 1: Orcon serves TLS itself

Give Orcon a certificate and its private key:

```yaml
publicUrl: https://orcon.example.com
http:
  address: ":8443"
  tls:
    certificateFile: /etc/orcon/tls/tls.crt
    keyFile: /etc/orcon/tls/tls.key
```

`http.address` is the address and port to listen on. The two keys take paths to files, and the files must be readable by the account that runs Orcon.

## Option 2: a reverse proxy or load balancer terminates TLS

Run Orcon on a private address and let a proxy or load balancer in front of it present the certificate. Set `publicUrl` to the HTTPS address that the proxy serves, and list the proxies that sit in front of Orcon in `http.trustedProxies`, as a list of CIDR ranges.

With several `api` processes behind a load balancer, the balancer needs no session affinity.

## Other listeners

The operations listener (`ops.address`, off by default) is separate. It serves only `/healthz`, `/readyz` and `/metrics`, and never the API, the web interface or trigger ingress. Do not expose it to the internet. See [Hardening checklist](../security/hardening-checklist.md).

## Shutdown

`http.shutdownTimeout` (30 seconds by default) is how long `api` waits for requests in progress when it receives a stop signal.
