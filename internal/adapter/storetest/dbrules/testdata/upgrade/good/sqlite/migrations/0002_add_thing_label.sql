-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

ALTER TABLE thing ADD COLUMN label TEXT;
UPDATE thing SET label = name;
