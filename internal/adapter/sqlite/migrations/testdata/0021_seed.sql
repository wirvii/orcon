-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Migration 0021 changes no schema on SQLite: the rows of the earlier seeds are the data the query reads.
SELECT 1 WHERE 0;
