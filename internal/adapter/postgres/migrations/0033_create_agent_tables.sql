-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The durable agent runs (SPEC-006 cut AI3, ADR-0060 sections 2 and 3). The SQLite counterpart of this migration
-- has the same tables and columns; see that file for the meaning of each one.

CREATE TABLE ai_agent_run (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  status text NOT NULL,
  step_count bigint NOT NULL,
  tool_call_count bigint NOT NULL,
  spend_microdollars bigint NOT NULL,
  final_text text,
  error_code text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_ai_agent_run PRIMARY KEY (id),
  CONSTRAINT uq_ai_agent_run_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_ai_agent_run_status CHECK (status IN ('RUNNING', 'SUCCEEDED', 'FAILED')),
  CONSTRAINT ck_ai_agent_run_counts CHECK (step_count >= 0 AND tool_call_count >= 0 AND spend_microdollars >= 0),
  CONSTRAINT fk_ai_agent_run_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

CREATE TABLE ai_agent_step (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  ai_agent_run_id uuid NOT NULL,
  step_number bigint NOT NULL,
  kind text NOT NULL,
  tool_call_key text NOT NULL,
  tool_name text NOT NULL,
  output_text text NOT NULL,
  is_error boolean NOT NULL,
  is_arguments_invalid boolean NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_ai_agent_step PRIMARY KEY (id),
  CONSTRAINT uq_ai_agent_step_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_agent_step_org_id_ai_agent_run_id_step_number UNIQUE (org_id, ai_agent_run_id, step_number),
  CONSTRAINT ck_ai_agent_step_kind CHECK (kind IN ('MODEL', 'TOOL')),
  CONSTRAINT fk_ai_agent_step_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_agent_step_ai_agent_run FOREIGN KEY (org_id, ai_agent_run_id) REFERENCES ai_agent_run (org_id, id) ON DELETE CASCADE
);

CREATE TABLE ai_model_call (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  ai_agent_step_id uuid NOT NULL,
  provider text NOT NULL,
  model text NOT NULL,
  input_token_count bigint NOT NULL,
  output_token_count bigint NOT NULL,
  cache_read_token_count bigint NOT NULL,
  cache_write_token_count bigint NOT NULL,
  reasoning_token_count bigint NOT NULL,
  cost_microdollars bigint NOT NULL,
  cost_status text NOT NULL,
  price_version text NOT NULL,
  finish_reason text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_ai_model_call PRIMARY KEY (id),
  CONSTRAINT uq_ai_model_call_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_model_call_org_id_ai_agent_step_id UNIQUE (org_id, ai_agent_step_id),
  CONSTRAINT ck_ai_model_call_provider_length CHECK (char_length(provider) BETWEEN 1 AND 100),
  CONSTRAINT ck_ai_model_call_model_length CHECK (char_length(model) BETWEEN 1 AND 200),
  CONSTRAINT ck_ai_model_call_counts CHECK (input_token_count >= 0 AND output_token_count >= 0 AND cache_read_token_count >= 0
    AND cache_write_token_count >= 0 AND reasoning_token_count >= 0 AND cost_microdollars >= 0),
  CONSTRAINT ck_ai_model_call_cost_status CHECK (cost_status IN ('EXACT', 'ESTIMATED', 'UNKNOWN')),
  CONSTRAINT fk_ai_model_call_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_model_call_ai_agent_step FOREIGN KEY (org_id, ai_agent_step_id) REFERENCES ai_agent_step (org_id, id) ON DELETE CASCADE
);

CREATE TABLE ai_agent_message (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  ai_agent_run_id uuid NOT NULL,
  sequence_number bigint NOT NULL,
  role text NOT NULL,
  part_kind text NOT NULL,
  call_key text NOT NULL,
  tool_name text NOT NULL,
  is_error boolean NOT NULL,
  content_text text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_ai_agent_message PRIMARY KEY (id),
  CONSTRAINT uq_ai_agent_message_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_agent_message_org_id_ai_agent_run_id_sequence_number UNIQUE (org_id, ai_agent_run_id, sequence_number),
  CONSTRAINT ck_ai_agent_message_role CHECK (role IN ('SYSTEM', 'USER', 'ASSISTANT', 'TOOL')),
  CONSTRAINT ck_ai_agent_message_part_kind CHECK (part_kind IN ('TEXT', 'TOOL_CALL', 'TOOL_RESULT')),
  CONSTRAINT fk_ai_agent_message_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_agent_message_ai_agent_run FOREIGN KEY (org_id, ai_agent_run_id) REFERENCES ai_agent_run (org_id, id) ON DELETE CASCADE
);

CREATE TABLE ai_conversation_message (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  thread_key uuid NOT NULL,
  conversation_key text NOT NULL,
  sequence_number bigint NOT NULL,
  role text NOT NULL,
  content_text text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_ai_conversation_message PRIMARY KEY (id),
  CONSTRAINT uq_ai_conversation_message_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_conversation_message_thread_key_sequence_number UNIQUE (thread_key, sequence_number),
  CONSTRAINT ck_ai_conversation_message_key_length CHECK (char_length(conversation_key) BETWEEN 1 AND 200),
  CONSTRAINT ck_ai_conversation_message_role CHECK (role IN ('SYSTEM', 'USER', 'ASSISTANT', 'TOOL')),
  CONSTRAINT fk_ai_conversation_message_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

-- Tenant isolation, as every organization table of this schema.
ALTER TABLE ai_agent_run ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_agent_run FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON ai_agent_run FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
ALTER TABLE ai_agent_step ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_agent_step FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON ai_agent_step FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
ALTER TABLE ai_model_call ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_model_call FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON ai_model_call FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
ALTER TABLE ai_agent_message ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_agent_message FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON ai_agent_message FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
ALTER TABLE ai_conversation_message ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_conversation_message FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON ai_conversation_message FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

GRANT SELECT, INSERT, UPDATE, DELETE ON ai_agent_run TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_agent_run TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_agent_step TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_agent_step TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_model_call TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_model_call TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_agent_message TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_agent_message TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_conversation_message TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_conversation_message TO orcon_engine;
