# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
# Preview image: builds the web editor and the Go binary from source on main (docs/engineering/preview.md).
# It is not the release image (build/release/Dockerfile). Build from the repository root:
#   docker build -f build/preview/Dockerfile .
FROM golang:1.26-bookworm AS build
ENV CGO_ENABLED=0 GOFLAGS=-trimpath GOTOOLCHAIN=local
RUN apt-get update \
 && apt-get install -y --no-install-recommends make curl ca-certificates \
 && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && corepack enable \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY . .
RUN pnpm install --frozen-lockfile
RUN make web build-go

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /src/bin/orcon /usr/local/bin/orcon
USER nonroot:nonroot
# The server writes its run files in the data directory; /tmp is writable on Cloud Run.
ENV ORCON_DATA_DIR=/tmp/orcon
EXPOSE 8080
# Cloud Run sends traffic to its default container port, 8080, on every interface, so the server listens on 0.0.0.0:8080.
ENTRYPOINT ["/usr/local/bin/orcon"]
CMD ["serve", "--role=standalone", "--http-address=0.0.0.0:8080"]
