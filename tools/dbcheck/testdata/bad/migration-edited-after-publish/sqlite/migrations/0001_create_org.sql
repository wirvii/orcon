-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self
CREATE TABLE org (
  id BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  name TEXT NOT NULL,
  CONSTRAINT pk_org PRIMARY KEY (id),
  CONSTRAINT ck_org_id_length CHECK (length(id) = 16)
) STRICT;
-- edited after publication
