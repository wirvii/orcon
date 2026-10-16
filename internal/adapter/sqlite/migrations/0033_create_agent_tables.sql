-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The durable agent runs (SPEC-006 cut AI3, ADR-0060 sections 2 and 3). A run is one item of one agent node
-- execution. Its steps are committed one by one; a model step has one model call row. The transcript is
-- append-only. Conversation memory keeps the messages of a conversation, shared by its runs.

CREATE TABLE ai_agent_run (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  status TEXT NOT NULL,
  step_count INTEGER NOT NULL,
  tool_call_count INTEGER NOT NULL,
  spend_microdollars INTEGER NOT NULL,
  final_text TEXT,
  error_code TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_ai_agent_run PRIMARY KEY (id),
  CONSTRAINT uq_ai_agent_run_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_ai_agent_run_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_ai_agent_run_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_ai_agent_run_status CHECK (status IN ('RUNNING', 'SUCCEEDED', 'FAILED')),
  CONSTRAINT ck_ai_agent_run_counts CHECK (step_count >= 0 AND tool_call_count >= 0 AND spend_microdollars >= 0),
  CONSTRAINT fk_ai_agent_run_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

CREATE TABLE ai_agent_step (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  ai_agent_run_id BLOB NOT NULL,
  step_number INTEGER NOT NULL,
  kind TEXT NOT NULL,
  tool_call_key TEXT NOT NULL,
  tool_name TEXT NOT NULL,
  output_text TEXT NOT NULL,
  is_error INTEGER NOT NULL,
  is_arguments_invalid INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_ai_agent_step PRIMARY KEY (id),
  CONSTRAINT uq_ai_agent_step_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_agent_step_org_id_ai_agent_run_id_step_number UNIQUE (org_id, ai_agent_run_id, step_number),
  CONSTRAINT ck_ai_agent_step_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_ai_agent_step_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_ai_agent_step_run_id_length CHECK (length(ai_agent_run_id) = 16),
  CONSTRAINT ck_ai_agent_step_kind CHECK (kind IN ('MODEL', 'TOOL')),
  CONSTRAINT ck_ai_agent_step_is_error CHECK (is_error IN (0, 1)),
  CONSTRAINT ck_ai_agent_step_is_arguments_invalid CHECK (is_arguments_invalid IN (0, 1)),
  CONSTRAINT fk_ai_agent_step_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_agent_step_ai_agent_run FOREIGN KEY (org_id, ai_agent_run_id) REFERENCES ai_agent_run (org_id, id) ON DELETE CASCADE
) STRICT;

-- The model call of a MODEL step: what was sent to the model, what it used and what it cost.
CREATE TABLE ai_model_call (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  ai_agent_step_id BLOB NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  input_token_count INTEGER NOT NULL,
  output_token_count INTEGER NOT NULL,
  cache_read_token_count INTEGER NOT NULL,
  cache_write_token_count INTEGER NOT NULL,
  reasoning_token_count INTEGER NOT NULL,
  cost_microdollars INTEGER NOT NULL,
  cost_status TEXT NOT NULL,
  price_version TEXT NOT NULL,
  finish_reason TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_ai_model_call PRIMARY KEY (id),
  CONSTRAINT uq_ai_model_call_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_model_call_org_id_ai_agent_step_id UNIQUE (org_id, ai_agent_step_id),
  CONSTRAINT ck_ai_model_call_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_ai_model_call_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_ai_model_call_step_id_length CHECK (length(ai_agent_step_id) = 16),
  CONSTRAINT ck_ai_model_call_provider_length CHECK (length(provider) BETWEEN 1 AND 100),
  CONSTRAINT ck_ai_model_call_model_length CHECK (length(model) BETWEEN 1 AND 200),
  CONSTRAINT ck_ai_model_call_counts CHECK (input_token_count >= 0 AND output_token_count >= 0 AND cache_read_token_count >= 0
    AND cache_write_token_count >= 0 AND reasoning_token_count >= 0 AND cost_microdollars >= 0),
  CONSTRAINT ck_ai_model_call_cost_status CHECK (cost_status IN ('EXACT', 'ESTIMATED', 'UNKNOWN')),
  CONSTRAINT fk_ai_model_call_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_model_call_ai_agent_step FOREIGN KEY (org_id, ai_agent_step_id) REFERENCES ai_agent_step (org_id, id) ON DELETE CASCADE
) STRICT;

-- The transcript of a run, one row per message part. Rows are written once and never changed.
CREATE TABLE ai_agent_message (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  ai_agent_run_id BLOB NOT NULL,
  sequence_number INTEGER NOT NULL,
  role TEXT NOT NULL,
  part_kind TEXT NOT NULL,
  call_key TEXT NOT NULL,
  tool_name TEXT NOT NULL,
  is_error INTEGER NOT NULL,
  content_text TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_ai_agent_message PRIMARY KEY (id),
  CONSTRAINT uq_ai_agent_message_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_agent_message_org_id_ai_agent_run_id_sequence_number UNIQUE (org_id, ai_agent_run_id, sequence_number),
  CONSTRAINT ck_ai_agent_message_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_ai_agent_message_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_ai_agent_message_run_id_length CHECK (length(ai_agent_run_id) = 16),
  CONSTRAINT ck_ai_agent_message_role CHECK (role IN ('SYSTEM', 'USER', 'ASSISTANT', 'TOOL')),
  CONSTRAINT ck_ai_agent_message_part_kind CHECK (part_kind IN ('TEXT', 'TOOL_CALL', 'TOOL_RESULT')),
  CONSTRAINT ck_ai_agent_message_is_error CHECK (is_error IN (0, 1)),
  CONSTRAINT fk_ai_agent_message_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_agent_message_ai_agent_run FOREIGN KEY (org_id, ai_agent_run_id) REFERENCES ai_agent_run (org_id, id) ON DELETE CASCADE
) STRICT;

-- The memory of a conversation: its messages in order, shared by every run of the conversation. thread_key is
-- derived from the organization and the conversation key, so one key names one conversation in one organization.
CREATE TABLE ai_conversation_message (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  thread_key BLOB NOT NULL,
  conversation_key TEXT NOT NULL,
  sequence_number INTEGER NOT NULL,
  role TEXT NOT NULL,
  content_text TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_ai_conversation_message PRIMARY KEY (id),
  CONSTRAINT uq_ai_conversation_message_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_conversation_message_thread_key_sequence_number UNIQUE (thread_key, sequence_number),
  CONSTRAINT ck_ai_conversation_message_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_ai_conversation_message_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_ai_conversation_message_thread_key_length CHECK (length(thread_key) = 16),
  CONSTRAINT ck_ai_conversation_message_key_length CHECK (length(conversation_key) BETWEEN 1 AND 200),
  CONSTRAINT ck_ai_conversation_message_role CHECK (role IN ('SYSTEM', 'USER', 'ASSISTANT', 'TOOL')),
  CONSTRAINT fk_ai_conversation_message_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;
