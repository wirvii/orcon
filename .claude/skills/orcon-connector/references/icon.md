<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Icon

Status: Accepted

This file covers the connector's icon file only. How icons are framed, sized and coloured in the product is decided by `orcon-ui` and is not repeated here. Rule IDs (`IC-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **IC-1** `icons/icon.svg`, the service's own mark used only to identify it (`../../orcon-ui/references/foundations.md#iconography`).
- **IC-2** Square `viewBox`; artwork centred with no padding baked in (the frame adds it).
- **IC-3** No scripts, event attributes, `foreignObject`, external references, embedded raster images or fonts; at most 8 KiB.
- **IC-4** Never n8n icons or artwork; never recolour or combine a third-party logo with the Orcon mark.
- **IC-5** Displayed only through `ConnectorIcon` (`<img>`), never inline.
- **IC-6** `icons/SOURCE`: one line naming where the icon came from: `simple-icons@<version>:<slug>`, `brand-kit:<url>` or `orcon-fallback`. Follow the brand's guidelines (clear space, no distortion).
- **IC-7** Optional `icons/icon-dark.svg`, only a variant the brand itself publishes (same rules IC-2..IC-5).
- **IC-8** Every official connector has a real brand icon; a generic placeholder (listed in `internal/connector/authoring/testdata/placeholder-icons.txt`) fails `orcon connector lint`.
