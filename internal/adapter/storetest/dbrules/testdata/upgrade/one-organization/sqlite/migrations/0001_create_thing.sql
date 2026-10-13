-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

CREATE TABLE org (id INTEGER NOT NULL PRIMARY KEY) STRICT;
CREATE TABLE thing (
  id INTEGER NOT NULL PRIMARY KEY,
  org_id INTEGER NOT NULL REFERENCES org (id),
  name TEXT NOT NULL
) STRICT;
