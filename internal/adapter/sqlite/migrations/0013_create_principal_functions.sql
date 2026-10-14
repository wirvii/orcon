-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The engine plane of PostgreSQL (orcon_engine functions) has no SQLite counterpart: the single
-- writer performs the same operations in the adapter, in one process. Deleting a principal clears its
-- actor columns there with plain batched statements.
