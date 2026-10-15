-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'flow_data: the rows of the previous version must all be PRODUCTION' WHERE (SELECT count(*) FROM flow_data WHERE namespace = 'PRODUCTION') <> 2;
SELECT 'node_test_data: the rows of the editor seed must survive the rebuild' WHERE (SELECT count(*) FROM node_test_data) <> 4;
SELECT 'flow_node_pin: the rows of the editor seed must survive the rebuild' WHERE (SELECT count(*) FROM flow_node_pin) <> 2;
SELECT 'test_run: the rows of the editor seed must survive the rebuild' WHERE (SELECT count(*) FROM test_run) <> 2;
SELECT 'test_run_seed: the rows of the editor seed must survive the rebuild' WHERE (SELECT count(*) FROM test_run_seed) <> 2;
SELECT 'effect_preview: the rows of the editor seed must survive the rebuild' WHERE (SELECT count(*) FROM effect_preview) <> 2;
SELECT 'flow_editing_session: the rows of the editor seed must survive the rebuild' WHERE (SELECT count(*) FROM flow_editing_session) <> 2;
