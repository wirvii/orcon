-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

SELECT 'label differs from name for thing ' || id FROM thing WHERE label IS NOT name;
