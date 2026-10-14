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
EXPOSE 8080
# Cloud Run sends traffic to its default container port, 8080, which is the port the server listens on.
ENTRYPOINT ["/usr/local/bin/orcon"]
CMD ["--role=standalone", "--addr=:8080"]
