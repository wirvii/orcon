# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
# Release image (ADR 0081). The base is pinned by digest; update it in a chore(deps) pull request.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY orcon /usr/local/bin/orcon
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/orcon"]
CMD ["serve", "--role=standalone", "--http-address=:8080"]
