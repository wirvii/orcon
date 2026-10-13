-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- name: ListThingsByName :many
-- orcon:scope org
-- orcon:hot
SELECT id FROM thing WHERE org_id = ? ORDER BY name, id LIMIT ?;
