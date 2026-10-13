-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- name: GetThingByName :one
-- orcon:scope org
-- orcon:hot
SELECT id FROM thing WHERE name = $1;
