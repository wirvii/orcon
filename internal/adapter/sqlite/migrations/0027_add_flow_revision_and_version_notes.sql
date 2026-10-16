-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The revision of the flow row (SPEC-005 Amendment 8 section 8-9): it starts at 0 and rises with every change of
-- the row (name, activation state, active version, soft delete, restore), but not with a draft save, so an
-- autosave never invalidates the ETag that activation, rollback, rename and delete carry in If-Match.
ALTER TABLE flow ADD COLUMN revision INTEGER NOT NULL DEFAULT 0
  CONSTRAINT ck_flow_revision_range CHECK (revision >= 0);

-- The release note of a publication or rollback (at most 500 characters) and, for a rollback, the number of the
-- version whose content was published again (SPEC-014 A7).
ALTER TABLE flow_version ADD COLUMN publication_note TEXT
  CONSTRAINT ck_flow_version_publication_note_length CHECK (length(publication_note) BETWEEN 1 AND 500);
ALTER TABLE flow_version ADD COLUMN rolled_back_from_version_number INTEGER
  CONSTRAINT ck_flow_version_rolled_back_from_version_number_range CHECK (rolled_back_from_version_number >= 1);
