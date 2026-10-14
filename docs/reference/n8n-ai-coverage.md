<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# n8n AI node inventory and Orcon mapping

Status: **Accepted** | Source: SPEC-006 Annex B, with ADR-0071 (accepted 2026-10-12 via SPEC-006)

Inventory of the n8n AI nodes and their counterpart in Orcon. Source: the public n8n documentation (docs.n8n.io: the `llms.txt` index and the AI Agent, Chat Trigger, Evaluation, human-in-the-loop and Agents preview pages), consulted on 2026-10-11. Only names and documented behaviour; no code. The final list is fixed against the n8n version pinned by the AI16 slice (gate of ADR-0071 §6).

Status values: **S** supported (`SUPPORTED`), **A** adapted (`ADAPTED`, with the difference reported), **U** unsupported (`UNSUPPORTED`, with a suggestion). On import, n8n option values are translated to the UPPER_SNAKE spelling of `orcon/flow@1` (ADR-0071 §4).

## Root nodes: agents and chains
| n8n | Orcon | Status | Note |
|---|---|---|---|
| AI Agent (Tools Agent; legacy variants Conversational, OpenAI Functions, ReAct, SQL, Plan and Execute) | `orcon.ai.agent@1` | S / A | Tools = S; ReAct, Conversational and OpenAI Functions → agent with native tool calls (A); SQL Agent → agent + read-only SQL tool of the database connector (A); Plan and Execute → agent with planning instructions (A). |
| Basic LLM Chain | `orcon.ai.llm@1` | S | |
| Question and Answer Chain | `orcon.ai.qa@1` | S | |
| Summarization Chain | `orcon.ai.summarize@1` | S | Durable `STUFF` / `MAP_REDUCE` / `REFINE`. |
| Information Extractor | `orcon.ai.extract@1` | S | |
| Text Classifier | `orcon.ai.classify@1` | S | One port per category. |
| Sentiment Analysis | `orcon.ai.sentiment@1` | S | |
| LangChain Code | — | U | Runs arbitrary LangChain JavaScript; suggest an agent or chain, or the Code node. |
| AI Transform | `orcon.code@1` | A | The generated code is kept; assisted generation in the editor (SPEC-006 Q13). |
| Guardrails | `orcon.ai.guardrails@1` | S | |
| Model Selector | `orcon.ai.modelSelector@1` | S | |
| MCP Client (node) | `orcon.mcp.client@1` | S | |
| Chat (send message / wait for reply) | `orcon.chat.send@1` | S | Durable wait. |
| Message an Agent (Agents preview) | call to an agent flow | A | SPEC-006 R-AG-13. |
| Evaluation (Set Outputs / Set Metrics / Check If Evaluating) | `orcon.evaluation.record@1` + `$execution.mode` | S | |
| OpenAI / Anthropic / Google Gemini / Mistral AI / Perplexity / Qwen Cloud (application nodes: text, image, audio, files, OCR) | operations of the provider connectors | S / A | Provider operations that are deprecated (for example Assistants) → A with a suggestion. |

## Triggers
| n8n | Orcon | Status |
|---|---|---|
| Chat Trigger | `orcon.chat@1` | S |
| MCP Server Trigger | `orcon.mcpServer@1` | S |
| Evaluation Trigger | `orcon.evaluation@1` | S |
| Microsoft Agent 365 Trigger | `appEvent` connector (BL-003 wave) | A |

## Sub-nodes: models
| n8n category | Nodes | Orcon |
|---|---|---|
| Chat Models | Anthropic, OpenAI, Google Gemini, Google Vertex, Azure OpenAI / Azure AI Foundry, AWS Bedrock, Mistral Cloud, Groq, DeepSeek, Ollama, Cohere, Databricks, OpenRouter, Vercel AI Gateway, xAI Grok, Moonshot Kimi, MiniMax, NVIDIA Nemotron, Qwen Cloud, Lemonade | Native providers of ADR-0061; the rest through `orcon.ai.openai-compatible` (the documented compatibility of each one is verified in the AI13 slice; without compatibility → its own connector in a later wave). |
| Language Models (non-chat) | Cohere, Ollama, Hugging Face Inference, Lemonade | Same provider in text mode (A: the chat API is used when the model offers it). |
| Embeddings | OpenAI, Azure OpenAI, Google Gemini, Google Vertex, Google PaLM (legacy), AWS Bedrock, Cohere, Mistral Cloud, Ollama, Hugging Face Inference, Lemonade, Oracle Database | `kind: EMBEDDING` of each provider; additional local `tfidf-hash-v1`. |
| Rerankers | Reranker Cohere | `kind: RERANKER`. |

## Sub-nodes: memory
| n8n | Orcon | Status |
|---|---|---|
| Simple Memory (Window Buffer) | persisted `WINDOW` memory | A (improvement: survives restarts, works across servers) |
| Postgres / Redis / MongoDB Chat Memory, Motorhead, Xata, Zep | `kind: MEMORY` (connectors) or import as Orcon's own memory (suggested) | S / A |
| Chat Memory Manager | `orcon.ai.memoryManager@1` | S |

## Sub-nodes: vector stores, loaders, splitters, retrievers, parsers
| n8n | Orcon | Status |
|---|---|---|
| Simple Vector Store (in memory) | `BUILTIN` store | A (persisted) |
| PGVector | `BUILTIN` on PostgreSQL or external `VECTOR_STORE` | S |
| Pinecone, Qdrant, Weaviate, Redis, Supabase, Milvus, MongoDB Atlas, Chroma, Azure AI Search, Oracle Database, Zep | `kind: VECTOR_STORE` | S (in waves) |
| Default Data Loader | `orcon.doc.load@1` | S |
| GitHub Document Loader | GitHub connector + `orcon.doc.load@1` | A |
| Character / Recursive Character / Token Text Splitter | `orcon.text.split@1` modes `CHARACTER` / `RECURSIVE_CHARACTER` / `TOKEN` | S |
| Vector Store Retriever | knowledge retrieval | S |
| MultiQuery Retriever | option `multiQuery` (RRF) | S |
| Contextual Compression Retriever | option `compress` | S |
| Workflow Retriever | flow as retriever | S |
| Structured Output Parser | `responseFormat: JSON_SCHEMA` | S |
| Auto-fixing Output Parser | `repairAttempts` | S |
| Item List Output Parser | option `splitList` | S |

## Sub-nodes: tools
| n8n | Orcon | Status |
|---|---|---|
| AI Agent Tool | inline sub-agent | S |
| Call n8n Workflow Tool | flow as a tool | S |
| MCP Client Tool | MCP tool | S |
| Databricks Genie MCP | MCP tool | S |
| Code Tool (Custom Code) | `code` tool (sandbox of ADR-0036) | S |
| HTTP Request Tool | `http` tool | S |
| Calculator, Think | built-in | S |
| Vector Store QA / Vector Store Tool | `knowledge_search` | S |
| Wikipedia, SearXNG, SerpApi, Wolfram\|Alpha | connectors as tools | S (in waves) |
| "Tool" variants of application nodes (for example Gmail Tool) + `$fromAI()` | connector operation as a tool + `$fromAI` | S |
| Human review in tools (n8n Chat, Slack, Discord, Telegram, Teams, Gmail, WhatsApp, Google Chat, Outlook) | `CHAT_USER` / `LINK` approval | S (with a public trigger, sensitive tools also require `ORCON` approval: floor of ADR-0068 §5) |

## Capabilities without a node
| n8n | Orcon |
|---|---|
| Agents (preview): session and episodic memory, skills, knowledge base, channels, sub-agents, versions | agent flows, `RETRIEVAL` memory, skills, knowledge stores, channel triggers, agent as a tool, ADR-0024 (SPEC-006 Q12) |
| Chat Hub (list of agents) | the editor's "Chat" page (`showInChat`) |
| Response streaming | ADR-0063 |
| Evaluations with metrics (correctness, helpfulness, string similarity, categorization, tools used, custom) | ADR-0069 (`CORRECTNESS`, `HELPFULNESS`, `STRING_SIMILARITY`, `CATEGORIZATION`, `TOOLS_USED`, recorded metrics) |
| n8n Assistant / AI Workflow Builder | outside SPEC-006 (Q13) |
