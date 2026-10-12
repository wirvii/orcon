# Owner review record template

Add one row per base component or primary screen template to the "Owner review status" table in `docs/design-system/README.md`. New rows, and rows whose approved visual baseline changed, start as `Pending`. Only the owner sets `Approved`.

Column meanings:

- **Item**: the component name as listed in `references/components.md#inventory`, or the template id and name (S1 to S9).
- **Kind**: `Base component` or `Screen template`.
- **Status**: `Pending`, `Changes requested` or `Approved`.
- **Date**: date of the last status change (YYYY-MM-DD); `—` while pending a first review.
- **Reviewed Storybook commit**: short commit hash of the static Storybook the owner reviewed.
- **Screenshots**: link to the captures or the CI artifact.
- **Notes**: requested changes, follow-up backlog items, or `—`.

Row to copy:

```
| <Item> | <Base component or Screen template> | Pending | — | <commit> | <link to captures> | — |
```
