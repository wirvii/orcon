-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self
CREATE TABLE org (
  id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  name varchar(255) NOT NULL,
  CONSTRAINT pk_org PRIMARY KEY (id)
);
