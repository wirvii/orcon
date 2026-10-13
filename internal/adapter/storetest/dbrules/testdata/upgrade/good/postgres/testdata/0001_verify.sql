-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

SELECT 'org without things' FROM org WHERE id NOT IN (SELECT org_id FROM thing);
