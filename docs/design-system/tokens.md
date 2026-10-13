<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Orcon design tokens v0.1

**Status: Proposed, implementation guidance, not a final approved palette.**

## Token architecture
Primitive tokens: color palette, spacing, typography, radii, shadow, opacity, motion. Semantic tokens: surface, text, border, focus, accent, success, warning, danger, pending, running, blocked, uncertain, completed. Component tokens reference semantic tokens only; no arbitrary hex values inside components.

## Proposed palette seeds
| Token | Light | Dark |
| --- | --- | --- |
| accent.500 | #16A34A | #22C55E |
| accent.teal | #0891B2 | #06B6D4 |
| surface.canvas | #F8FAFC | #0F172A |
| surface.panel | #FFFFFF | #10221B |
| text.primary | #0F172A | #F8FAFC |
| text.secondary | #475569 | #CBD5E1 |
| border.default | #CBD5E1 | #334155 |
| focus.ring | #0284C7 | #7DD3FC |

Contrast ratios and interactive combinations **must be audited** for WCAG 2.2 AA before acceptance. Use non-color status cues.

## Layout and spacing
Base unit 4px; proposed scale 0, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64. Radii 4, 8, 12, 16px. Canvas layout must remain readable on common laptop displays. Workflow inspector, minimap, palette and execution controls should adapt to smaller viewports without obscuring critical status.

## Typography
UI: Inter or system sans, pending licensed font choice. Code/expressions: system monospace. Suggested text sizes 12, 14, 16, 20, 24, 32px; line-height 1.4–1.6 for prose, appropriate tighter headings. Minimum comfortable control targets and focus-visible outlines.

## Motion
Use motion sparingly: 120–200ms UI feedback, reduced-motion preference honored. Never animate large graphs continuously without performance controls. Execution status should not rely on animation alone.

## State semantics
Pending, ready, running, waiting, completed, failed, cancelled, blocked and UNCERTAIN require unique icons/labels and screen-reader text. UNCERTAIN is not FAILED or SUCCEEDED. Destructive actions need confirmation and visible impact.
