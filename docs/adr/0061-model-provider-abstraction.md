<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0061 — Model provider abstraction, normalization, structured outputs and fallback

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: UPPER_SNAKE values per ADR-0080 §3)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
A2: providers Anthropic, OpenAI, Google Gemini, Azure OpenAI, AWS Bedrock, Mistral, Groq, DeepSeek, local models (Ollama) and a generic OpenAI-compatible provider; one uniform abstraction (switching provider = one setting); fallback model on outage/429; streaming. ADR-0029 §9 reserved connector kinds `MODEL`, `EMBEDDING`; network and credentials follow ADR-0029 §6, §8.

## Decision

### 1. Providers are official native connectors
`kind: MODEL` (and `EMBEDDING`, `RERANKER` where offered) native Go connectors: `orcon.ai.anthropic`, `orcon.ai.openai`, `orcon.ai.gemini` (Google AI and Vertex AI endpoints), `orcon.ai.azure-openai`, `orcon.ai.bedrock`, `orcon.ai.mistral`, `orcon.ai.groq`, `orcon.ai.deepseek`, `orcon.ai.ollama`, `orcon.ai.openai-compatible` (base URL + credential). HTTP clients are written in-repo on the egress client; Bedrock signing uses AWS SigV4 (aws-sdk-go-v2, Apache-2.0). Third parties may ship `wasm` model connectors with the same interface (ADR-0032).

### 2. Canonical interface
```go
type ModelProvider interface {
    Capabilities(model string) Capabilities
    Generate(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request, sink DeltaSink) (Response, error)
    Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error)
}
```
`Request`: `messages[]` (roles `SYSTEM|USER|ASSISTANT|TOOL`) of parts (`text`, `image`, `file` (BlobRef), `toolCall`, `toolResult`, `reasoning` (opaque provider blob, round-tripped)), `tools[]` (name, description, JSON Schema subset common to providers), `toolChoice` (`AUTO|NONE|REQUIRED|{name}`), `responseFormat` (`TEXT|JSON|JSON_SCHEMA{schema, strict}`), `maxOutputTokens`, `temperature`, `topP`, `stop`, `reasoning{effort|budgetTokens}`, `cacheHints`, `ext` (provider-specific, namespaced). `Response`: parts, `toolCalls[]{id, name, arguments}`, `finishReason` (`STOP|LENGTH|TOOL_CALLS|CONTENT_FILTER|REFUSAL|ERROR`), `usage{input, output, cacheRead, cacheWrite, reasoning}`, `providerRequestId`, `modelVersion`. Provider wire values exist only inside each adapter.
`Capabilities`: tools, parallelTools, vision, files, jsonSchema, reasoning, promptCache, streaming, contextTokens, maxOutputTokens, embeddingDims.

### 3. Normalization rules
- Tool names sanitized to `^[a-zA-Z0-9_-]{1,64}$` with a recorded mapping; arguments parsed as JSON and validated against the tool schema before dispatch; invalid arguments return an error tool result to the model (a step); two consecutive invalid calls to the same tool fail the run.
- Provider-specific message constraints (alternation, system placement, tool-result grouping) are handled inside each adapter; the canonical transcript never changes.
- Finish reasons and errors are classified into ADR-0011 classes: 429/5xx/529/timeouts → transient; auth/validation → permanent; content refusal → permanent with `REFUSAL` detail.

### 4. Structured outputs
In order of preference: native JSON-schema mode; forced `final_answer` tool with the schema; instruction + validation + bounded repair (`repairAttempts`, default 2, each a recorded step).

### 5. Fallback and circuit breaking
`model: {provider, model, credential, params}` plus ordered `fallbacks[]`. Switch to the next on 429 with `Retry-After > maxRetryAfter` (default 10 s) or after retries, 5xx, 529/overloaded, timeouts and connection errors; never on validation, refusal or credential errors. Publication validates that every fallback supports the capabilities the node uses. A per-`(provider, credential)` circuit breaker uses `limit_bucket.blocked_until_at` (ADR-0014; column name per ADR-0077) so all workers skip a failing endpoint. The model that answered is recorded on the step.

### 6. Rate limits
Requests/min and tokens/min declared in the manifest map to level-4 buckets (ADR-0014) keyed by credential; operators may adjust.

### 7. Test provider
`test/mockllm`: scripted responses per case, tool calls, deltas, latency, error injection (429, 5xx, 529, timeouts, disconnects mid-stream), usage, and assertion hooks (e.g. "no sensitive value ever received").

## Consequences
Switching provider is one setting; capability mismatches are caught at publication; outages degrade to fallbacks without duplicate billing of recorded steps.

## Alternatives considered
LangChain-style chains in a JavaScript runtime (rejected: second runtime, vision.md); a single OpenAI-compatible adapter for every provider (rejected: loses native tool, caching and reasoning semantics of Anthropic, Gemini, Bedrock); vendor SDKs everywhere (rejected: dependency weight and licence review per SDK; kept only where signing requires it).
