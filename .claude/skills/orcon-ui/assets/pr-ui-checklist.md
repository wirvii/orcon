## UI checklist

<!-- Paste this block into the description of every pull request that changes apps/web or packages/js/{tokens,ui,lint-config}. -->

- [ ] I loaded the `orcon-ui` skill and followed `references/component-checklist.md` or `references/screen-checklist.md`.
- [ ] `sh .agents/skills/orcon-ui/scripts/verify.sh` exits 0.
- [ ] Every new or changed component declares `states` and `pairs` in `meta.ts` and has one story per state plus `Matrix`.
- [ ] Checked in light and dark, comfortable and compact, mobile and desktop, and the pseudo-locale `xx-LS`.
- [ ] No hex, arbitrary values, default Tailwind palette or hard-coded strings.
- [ ] Status is icon + text; red, amber and violet only on status; UNCERTAIN never shown as failure or success.
- [ ] Screenshot review findings table attached (see below).
- [ ] Manual keyboard and screen-reader pass recorded (see below).
- [ ] Base components or primary screens touched have a `Pending` row in "Owner review status" of `docs/design-system/README.md`.
- [ ] If the skill changed: re-projected with `sh .agents/skills/orcon-ui/scripts/project.sh` and `sh .agents/skills/orcon-ui/validation/run.sh` exits 0.
- [ ] Comments, stories, commit messages and this description use neutral contributor wording (no coding-agent product names, no AI authorship trailers).

### Screenshot review

| Capture | Criterion | Finding | Severity (blocker / major / minor) | Action |
|---|---|---|---|---|
| | | | | |

### Manual accessibility pass

| Screen or component | Keyboard | Screen reader (tool and version) | Zoom 200 % / reflow | Reduced motion | Findings |
|---|---|---|---|---|---|
| | | | | | |
