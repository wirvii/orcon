<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Connector pull request checklist

Paste this block into the pull request description and tick every line that applies. Rule ids point to `.agents/skills/orcon-connector/references/`.

```markdown
### Connector checklist

- [ ] 1. Declarative by default; a native connector names a gap from `docs/reference/declarative-gaps.md` in `authoring.json` (KD-2, KD-3)
- [ ] 2. Operations are `<resource>.<verb>` with a singular resource and a vocabulary verb, or a written reason (NM-3 to NM-5)
- [ ] 3. Every `getMany` offers `returnAll` and `limit`; the record id parameter is `<resource>Id`; outputs have a top-level `id` (NM-7, NM-8, NM-10)
- [ ] 4. Connector-defined dates are RFC 3339 in UTC; provider fields are returned unchanged (NM-11)
- [ ] 5. Every user-visible string is in `i18n/en.json` and `i18n/es.json` (NM-12)
- [ ] 6. Network access only to exact domains from the provider documentation, over HTTPS (MF-5 to MF-7)
- [ ] 7. Credentials reach the connector as a handle; every credential type has a test operation; OAuth scopes are minimal (CR)
- [ ] 8. Every write declares its safety level; `UNSAFE` writes declare `manualCheck` (SF-1, SF-4)
- [ ] 9. Errors use the 14 codes; ambiguous writes without an idempotency key are `UNCERTAIN`; enumerated values are UPPER_SNAKE (ER-4, NM-13)
- [ ] 10. Every operation has a recorded example; cassettes are scrubbed and pass the detector (TE-2 to TE-4)
- [ ] 11. Fault tests prove retries only when safe and UNCERTAIN for ambiguous writes (TE-6)
- [ ] 12. The version bump matches the detected change; change descriptors and changelog in English and Spanish (VR)
- [ ] 13. Documentation regenerated; connection guide written in both languages (DC)
- [ ] 14. Icon: square SVG, no scripts or external references, at most 8 KiB, never n8n artwork (IC)
- [ ] 15. Every replaced n8n type and typeVersion mapped completely; no n8n code, text or images (N8)
- [ ] 16. No build-assistant names or AI authorship trailers; AI-provider connectors and pages only under approved provenance paths
- [ ] 17. Skill edits only under `.agents/skills/orcon-connector/`, followed by `scripts/project.sh`; never `mneme skills pin`

### Commands run

- [ ] `make connectors-check` — result:
- [ ] `make connectors-version-check BASE=origin/main` — result:
- [ ] `sh .agents/skills/orcon-connector/scripts/connector-change-check.sh <folder>` — result:
- [ ] Known limitations:
```
