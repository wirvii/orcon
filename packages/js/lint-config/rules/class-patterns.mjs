// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Regular expressions (as strings) that reject Tailwind classes the Orcon
// design system forbids. They are applied to one class token at a time by
// eslint-plugin-better-tailwindcss `no-restricted-classes`, so variants such as
// `hover:` or `md:` are part of the token and handled by the leading group.

const VARIANTS = '(?:^|:)!?-?';

// Arbitrary values (`w-[13px]`, `bg-[#fff]`) and arbitrary properties
// (`[mask-type:alpha]`). Arbitrary *variants* (`data-[state=open]:`,
// `[&_svg]:`) are not values and stay allowed.
export const arbitraryValue = `${VARIANTS}(?:[a-z][a-z0-9-]*-\\[[^\\]]+\\]|\\[[a-z-]+:[^\\]]+\\])(?:/\\S+)?$`;

const hues =
  'slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose';
const colorUtilities =
  'bg|text|border|border-[xytrblse]|ring|ring-offset|outline|fill|stroke|from|via|to|divide|decoration|accent|caret|shadow|placeholder';

// Tailwind default palette (`bg-red-500`, `text-slate-600`) and the keyword
// colors `white` and `black`, which the token theme removes.
export const defaultPalette = `${VARIANTS}(?:${colorUtilities})-(?:(?:${hues})(?:-(?:50|[1-9]00|950))?|white|black)(?:/\\d+)?$`;

const spacingUtilities =
  'p|px|py|pt|pr|pb|pl|ps|pe|m|mx|my|mt|mr|mb|ml|ms|me|gap|gap-x|gap-y|space-x|space-y|inset|inset-x|inset-y|top|right|bottom|left|start|end|w|h|size|min-w|min-h|max-w|max-h|basis|scroll-m|scroll-p';

// The spacing scale is 0, 1, 2, 3, 4, 5, 6, 8, 10, 12 and 16 (steps of 4 px).
// Every other numeric step, including half steps, is off the scale.
export const offScaleSpacing = `${VARIANTS}(?:${spacingUtilities})-(?:7|9|11|1[3-5]|1[7-9]|[2-9]\\d|\\d{3,}|\\d+\\.\\d+)$`;

export const restrictedClasses = [
  {
    pattern: arbitraryValue,
    message:
      'Arbitrary Tailwind values are not allowed. Use a token utility or create a token in @orcon/tokens.',
  },
  {
    pattern: defaultPalette,
    message:
      'Tailwind default palette classes are not allowed. Use a semantic token utility such as bg-surface or text-fg-muted.',
  },
  {
    pattern: offScaleSpacing,
    message:
      'Spacing must be on the scale (0, 1, 2, 3, 4, 5, 6, 8, 10, 12, 16). Use a scale step or a density token.',
  },
];
