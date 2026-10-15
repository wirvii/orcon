-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The execution level of the concurrency limits (SPEC-001 C11, ADR-0014 section 2.2). The claim itself is the
-- ClaimTokens query, which SQLite runs under its single writer; the schema does not change. The PostgreSQL
-- counterpart is the claim_tokens function of the same number.
