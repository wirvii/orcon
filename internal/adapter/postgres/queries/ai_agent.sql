-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the durable agent runs (ai_agent_run, ai_agent_step, ai_model_call, ai_agent_message,
-- ai_conversation_message; SPEC-006 AI3). The SQLite file of the same name holds the same query names.
-- A committed step is one transaction: the step, its model call, its transcript rows and the run counters.

-- name: InsertAIAgentRunIfAbsent :exec
-- orcon:scope org
INSERT INTO ai_agent_run (id, org_id, status, step_count, tool_call_count, spend_microdollars, final_text, error_code, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), 'RUNNING', 0, 0, 0, NULL, NULL, sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (id) DO NOTHING;

-- name: GetAIAgentRun :one
-- orcon:scope org
SELECT status, step_count, tool_call_count, spend_microdollars, final_text, error_code
FROM ai_agent_run
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: UpdateAIAgentRunCounters :exec
-- orcon:scope org
UPDATE ai_agent_run
SET step_count = step_count + 1,
    tool_call_count = tool_call_count + sqlc.arg(tool_calls),
    spend_microdollars = spend_microdollars + sqlc.arg(spend_microdollars),
    updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: MarkAIAgentRunSucceeded :exec
-- orcon:scope org
UPDATE ai_agent_run
SET status = 'SUCCEEDED', final_text = sqlc.arg(final_text), updated_at = sqlc.arg(updated_at)
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);

-- name: InsertAIAgentStep :exec
-- orcon:scope org
INSERT INTO ai_agent_step (id, org_id, ai_agent_run_id, step_number, kind, tool_call_key, tool_name, output_text, is_error,
  is_arguments_invalid, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(ai_agent_run_id), sqlc.arg(step_number), sqlc.arg(kind), sqlc.arg(tool_call_key),
  sqlc.arg(tool_name), sqlc.arg(output_text), sqlc.arg(is_error), sqlc.arg(is_arguments_invalid), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: InsertAIModelCall :exec
-- orcon:scope org
INSERT INTO ai_model_call (id, org_id, ai_agent_step_id, provider, model, input_token_count, output_token_count, cache_read_token_count,
  cache_write_token_count, reasoning_token_count, cost_microdollars, cost_status, price_version, finish_reason, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(ai_agent_step_id), sqlc.arg(provider), sqlc.arg(model), sqlc.arg(input_token_count),
  sqlc.arg(output_token_count), sqlc.arg(cache_read_token_count), sqlc.arg(cache_write_token_count), sqlc.arg(reasoning_token_count),
  sqlc.arg(cost_microdollars), sqlc.arg(cost_status), sqlc.arg(price_version), sqlc.arg(finish_reason), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: ListAIAgentSteps :many
-- orcon:scope org
SELECT id, step_number, kind, tool_call_key, tool_name, output_text, is_error, is_arguments_invalid
FROM ai_agent_step
WHERE org_id = sqlc.arg(org_id) AND ai_agent_run_id = sqlc.arg(ai_agent_run_id)
ORDER BY step_number, id
LIMIT sqlc.arg(max_rows);

-- name: ListAIModelCallsForRun :many
-- orcon:scope org
SELECT mc.ai_agent_step_id, mc.provider, mc.model, mc.input_token_count, mc.output_token_count, mc.cache_read_token_count,
  mc.cache_write_token_count, mc.reasoning_token_count, mc.cost_microdollars, mc.cost_status, mc.price_version, mc.finish_reason
FROM ai_model_call mc
JOIN ai_agent_step s ON s.org_id = mc.org_id AND s.id = mc.ai_agent_step_id
WHERE mc.org_id = sqlc.arg(org_id) AND s.ai_agent_run_id = sqlc.arg(ai_agent_run_id)
ORDER BY mc.id
LIMIT sqlc.arg(max_rows);

-- name: InsertAIAgentMessage :exec
-- orcon:scope org
INSERT INTO ai_agent_message (id, org_id, ai_agent_run_id, sequence_number, role, part_kind, call_key, tool_name, is_error,
  content_text, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(ai_agent_run_id), sqlc.arg(sequence_number), sqlc.arg(role), sqlc.arg(part_kind),
  sqlc.arg(call_key), sqlc.arg(tool_name), sqlc.arg(is_error), sqlc.arg(content_text), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: ListAIAgentMessages :many
-- orcon:scope org
SELECT sequence_number, role, part_kind, call_key, tool_name, is_error, content_text
FROM ai_agent_message
WHERE org_id = sqlc.arg(org_id) AND ai_agent_run_id = sqlc.arg(ai_agent_run_id)
ORDER BY sequence_number, id
LIMIT sqlc.arg(max_rows);

-- name: ListAIConversationMemory :many
-- orcon:scope org
SELECT sequence_number, role, content_text
FROM ai_conversation_message
WHERE org_id = sqlc.arg(org_id) AND thread_key = sqlc.arg(thread_key)
ORDER BY sequence_number DESC, id
LIMIT sqlc.arg(max_rows);

-- name: GetAIConversationLastSequence :one
-- orcon:scope org
SELECT CAST(COALESCE(MAX(sequence_number), 0) AS bigint) AS last_sequence
FROM ai_conversation_message
WHERE org_id = sqlc.arg(org_id) AND thread_key = sqlc.arg(thread_key);

-- name: InsertAIConversationMessage :exec
-- orcon:scope org
INSERT INTO ai_conversation_message (id, org_id, thread_key, conversation_key, sequence_number, role, content_text, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(thread_key), sqlc.arg(conversation_key), sqlc.arg(sequence_number), sqlc.arg(role),
  sqlc.arg(content_text), sqlc.arg(created_at), sqlc.arg(updated_at));
