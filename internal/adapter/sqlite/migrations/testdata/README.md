<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Migration test data (sqlite)

Each migration `NNNN_<name>.sql` ships here, for this dialect, two files
(migrations.md MG-11):

- `NNNN_seed.sql`: representative rows valid at version NNNN, with at least two
  organizations and a row in every organization-scoped table the migration
  creates;
- `NNNN_verify.sql`: SELECT statements that return zero rows when the seeded
  data survived the upgrade; a returned row is a failure.

`dbrules.RunUpgradeTests` applies the migrations in order with their seeds,
then runs the verify file of the migration under test.
