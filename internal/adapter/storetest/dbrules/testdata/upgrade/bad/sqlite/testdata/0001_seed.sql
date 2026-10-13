-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

INSERT INTO org (id) VALUES (1), (2);
INSERT INTO thing (id, org_id, name) VALUES (1, 1, 'alpha'), (2, 1, 'beta'), (3, 2, 'gamma');
