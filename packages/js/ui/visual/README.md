<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Visual regression

The visual suite compares a screenshot of every Matrix story and every screen template with a committed
baseline, in light and dark, comfortable and compact, on a phone (390 x 844) and a desktop (1440 x 900), and
on a tablet (1024 x 768) for the flow editor. A second project, `pseudo`, opens the same targets in the
long-string pseudo-locale and fails when text overflows a box that does not declare truncation.

## Baselines come from the pinned container only

Fonts and rasterization differ per operating system, so a screenshot made on a developer machine is not a
valid baseline and must never be committed. Baselines are made, and compared, in the pinned Playwright
container (`mcr.microsoft.com/playwright:v1.64.0-noble`, the architecture of the CI runners):

```sh
pnpm --filter @orcon/ui test:visual:docker                          # compare with the baselines
pnpm --filter @orcon/ui test:visual:docker -- --update-snapshots    # regenerate them
pnpm --filter @orcon/ui test:visual:docker -- --project=pseudo      # only the text expansion check
```

`test:visual:docker` builds Storybook on the host, then runs the suite in the container with the built
`storybook-static` mounted read only. Baselines are written to `visual/__screenshots__`.

`pnpm --filter @orcon/ui test:visual` runs the same suite on the host. Use it to look at a change quickly; its
screenshots are not comparable with the committed baselines.

## Changing a baseline

If a visual change is intended, regenerate the baselines in the container and review the image diff. If the
component or screen is already marked `Approved` in "Owner review status" of `docs/design-system/README.md`,
its row goes back to `Pending` and the owner reviews it again.

## Files

- `playwright.config.ts`: projects `visual` and `pseudo`, threshold `maxDiffPixelRatio 0.001`, reduced motion, UTC.
- `targets.ts`: reads the built Storybook index to list the targets.
- `visual.spec.ts`, `pseudo.spec.ts`: the two suites.
- `serve.mjs`: serves `storybook-static` without any dependency.
- `run-docker.sh`: runs the suite in the pinned container.
