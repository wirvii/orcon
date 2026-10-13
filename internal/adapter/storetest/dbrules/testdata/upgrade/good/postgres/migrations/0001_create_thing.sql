-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

CREATE TABLE org (id bigint NOT NULL PRIMARY KEY);
CREATE TABLE thing (
  id bigint NOT NULL PRIMARY KEY,
  org_id bigint NOT NULL REFERENCES org (id),
  name text NOT NULL
);
