-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
INSERT INTO instance_key (id, key_level, key_version, wrapped, master_key_reference, status, created_at, updated_at) VALUES (X'00000016000070008000000000000001', 'INSTANCE', 1, X'0102030405060708090a0b0c0d0e0f10', 'file:seed', 'ACTIVE', 1760000000000000, 1760000000000000);
INSERT INTO instance_key_canary (id, key_level, key_version, probe, created_at, updated_at) VALUES (X'00000016000070008000000000000002', 'MASTER', 1, X'aa01', 1760000000000000, 1760000000000000);
INSERT INTO instance_key_canary (id, key_level, key_version, probe, created_at, updated_at) VALUES (X'00000016000070008000000000000003', 'INSTANCE', 1, X'aa02', 1760000000000000, 1760000000000000);
INSERT INTO org_key (id, org_id, key_version, wrapped, master_key_reference, status, probe, created_at, updated_at) VALUES (X'00000016000070008000000000000004', X'00000003000070008000000100000001', 1, X'1112131415161718191a1b1c1d1e1f20', 'file:seed', 'ACTIVE', X'bb01', 1760000000000000, 1760000000000000);
INSERT INTO org_key (id, org_id, key_version, wrapped, master_key_reference, status, probe, created_at, updated_at) VALUES (X'00000016000070008000000000000005', X'00000003000070008000000200000001', 1, X'2122232425262728292a2b2c2d2e2f30', 'file:seed', 'ACTIVE', X'bb02', 1760000000000000, 1760000000000000);
