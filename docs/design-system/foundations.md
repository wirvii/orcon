# Orcon — Design System Foundations

Status: **Visual direction approved; implementation tokens proposed** | Version: 0.2

## Canonical visual reference
See [brand/README.md](../../brand/README.md). The product owner approved **two original visual concept boards** showing a semi-3D green orc engineer with tied dark hair, glasses, beard and dark hoodie, together with a geometric green-to-teal logo and the Orcon wordmark. Any design that materially differs from these boards must be rejected, even if it is technically valid SVG.

## Brand
Name: **Orcon**. Expanded: **Orchestration Controller**. Tagline: **Build. Orchestrate. Execute. Your way.** Keep mascot and geometric mark separate. Mascot art is detailed, textured and polished; not a flat cartoon or primitive SVG approximation.

## Approved reference colors (primary board)
- Green #22C55E
- Dark green #16A34A
- Cyan #06B6D4
- Navy #0F172A
- Slate #64748B

These are **visual reference colors**, not an accessibility-certified semantic token system. Light backgrounds and text must meet contrast requirements. Earlier exploratory #10221B/#BAF4C9 palettes are superseded as brand references.

## System requirements
Complete primitive and semantic tokens; light/dark themes; spacing, typography, radii, elevation, motion; focus, hover, disabled, error and loading states; responsive app shell; workflow canvas, nodes, connections, inspector, execution debugger, migration wizard and approvals. Components must reference semantic tokens. Target WCAG 2.2 AA; accessible keyboard alternative to graph drag/drop, descriptive node/edge labels, reduced-motion support and no color-only status.

## Required deliverables
1. Commit the **original two approved concept boards** under brand/references/.
2. Create logo/mascot production masters based on those boards; review side-by-side.
3. Publish detailed component specifications and a token package with contrast tests.
4. Do not declare the Brand Kit complete until original assets and approved exports exist.

## Important
Previous substitute SVGs did not match approved visuals and were removed. Agents must not regenerate arbitrary replacements or mistake the preliminary scaffold for approved design.
