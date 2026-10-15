-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
INSERT INTO instance_key (key_level, key_version, wrapped, master_key_reference, status, created_at, updated_at) VALUES ('INSTANCE', 1, decode('0102030405060708090a0b0c0d0e0f10', 'hex'), 'file:seed', 'ACTIVE', '2025-10-09T08:53:20Z', '2025-10-09T08:53:20Z');
INSERT INTO instance_key_canary (key_level, key_version, probe, created_at, updated_at) VALUES ('MASTER', 1, decode('aa01', 'hex'), '2025-10-09T08:53:20Z', '2025-10-09T08:53:20Z');
INSERT INTO instance_key_canary (key_level, key_version, probe, created_at, updated_at) VALUES ('INSTANCE', 1, decode('aa02', 'hex'), '2025-10-09T08:53:20Z', '2025-10-09T08:53:20Z');
INSERT INTO org_key (org_id, key_version, wrapped, master_key_reference, status, probe, created_at, updated_at) VALUES ('00000003-0000-7000-8000-000100000001', 1, decode('1112131415161718191a1b1c1d1e1f20', 'hex'), 'file:seed', 'ACTIVE', decode('bb01', 'hex'), '2025-10-09T08:53:20Z', '2025-10-09T08:53:20Z');
INSERT INTO org_key (org_id, key_version, wrapped, master_key_reference, status, probe, created_at, updated_at) VALUES ('00000003-0000-7000-8000-000200000001', 1, decode('2122232425262728292a2b2c2d2e2f30', 'hex'), 'file:seed', 'ACTIVE', decode('bb02', 'hex'), '2025-10-09T08:53:20Z', '2025-10-09T08:53:20Z');
