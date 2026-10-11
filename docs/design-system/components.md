# Orcon component and UX specification v0.1

**Status: Proposed.** All components must support keyboard navigation, accessible names, loading/empty/error states, dark/light themes and reduced motion.

## Foundation components
Button (primary/secondary/tertiary/danger, size, disabled/loading), IconButton, Input, Textarea, Select/Combobox, Checkbox, Radio, Switch, FormField, Tooltip, Popover, Menu, Dialog, Drawer, Tabs, Breadcrumb, Table, Pagination, Badge, Toast, Banner, Skeleton, EmptyState, ErrorBoundary.

## Application shell
Global navigation, workspace switcher, environment selector, workflows list, editor, executions, credentials, integrations, settings, help and command palette. Avoid assuming enterprise-only features. Keep separation between authoring and operational views.

## Workflow editor
React Flow canvas with zoom/pan, minimap, accessible keyboard graph editing, node palette/search, drag/drop, node inspector, versioning, validation, explicit save/publish/activate states, undo/redo, subworkflow navigation and conflict handling. Edges must have meaningful port types and semantic validation. Nodes must distinguish trigger, action, transform, control, AI and group categories. Do not infer parallelism from visual branching without defined runtime semantics.

## Execution/debugging
Execution list, timeline, per-node attempts, input/output preview with secret redaction, retry/recovery evidence, logs and trace correlation. Show running/waiting/completed/failed/cancelled and **UNCERTAIN external operation** states. Provide actionable recovery UI; never offer unsafe blind retry as the primary action.

## External operations UX
Contract inspector shows effect, idempotency, verification, compensation, trust level and failure policy. Activation validation blocks unsafe automatic side effects. Consistency groups may be visually bounded, but 2PC/TCC/Saga are optional future features. Explicit supervised approval is recorded with actor, timestamp, exact operation arguments and scope.

## n8n migration UX
Import wizard shows source version, node mapping, exact/adapted/unsupported/manual classifications, credential re-binding, graph diffs and activation blockers. No automatic activation of unverified side effects.

## Accessibility and performance
Target WCAG 2.2 AA. Provide keyboard alternatives to drag/drop and connecting ports, semantic announcements for status changes, visible focus, contrast audits and screen-reader-friendly node descriptions. Virtualize large node catalogs and optimize canvas rendering for large graphs; define measurable performance acceptance tests.
