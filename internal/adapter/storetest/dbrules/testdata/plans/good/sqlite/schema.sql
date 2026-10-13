-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

CREATE TABLE thing (
  id INTEGER NOT NULL,
  org_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_thing PRIMARY KEY (id)
) STRICT;
CREATE INDEX ix_thing_org_id_created_at_id ON thing (org_id, created_at, id);
