-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The engine plane of PostgreSQL has no SQLite counterpart: the clearing function of migration 0013 is
-- replaced there to include the actor columns of the editor tables. SQLite clears the same columns with
-- the batched statements of the adapter (UpdateFlowDraftUpdatedByPrincipalToNull and the others in
-- queries/identity.sql), so this migration changes nothing.
