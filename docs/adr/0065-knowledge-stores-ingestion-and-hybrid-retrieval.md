<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0065 — Knowledge stores, ingestion and hybrid retrieval on SQLite and PostgreSQL

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: table names per ADR-0077; metadata filters on real columns per the Go rulebook DA-3 as amended; `reindex` action per ADR-0080 §2; UPPER_SNAKE values per ADR-0080 §3) (amended 2026-10-13: stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
A3: knowledge in Orcon's database by default (PostgreSQL via pgvector, SQLite via in-Go vector search), external vector stores optional (n8n's set), ingestion as normal durable flows, per-organization isolation; reuse mneme's hybrid retrieval: FTS5 BM25 + Go cosine + Reciprocal Rank Fusion (`internal/scoring/rrf.go`, k = 60, per-backend weights) and the offline TF-IDF embedder (`internal/embed/tfidf.go`); pluggable in-memory HNSW later. Verified: `modernc.org/sqlite` v1.53.0 compiles SQLite with `SQLITE_ENABLE_FTS5` on all 19 generated platform builds; mneme uses FTS5 with it without CGO or build tags.

## Decision

### 1. Stores
`knowledge_store(org_id, id, project_id, name, backend: BUILTIN|EXTERNAL, connector_reference?, embedding_document, lexical_document, chunking_document (each with its _format column), status: READY|REINDEX_REQUIRED|REINDEXING)` (embedding configuration `{provider, model, dims, credential}` or `tfidf-hash-v1`; lexical `{sqliteTokenizer, pgRegconfig}`; the embedding model is also a real column `embedding_model` because queries filter by it); `knowledge_document(org_id, id, knowledge_store_id, source_uri, title, content_hash, version_number, metadata_document, metadata_document_format, status, ingested_at)`; `knowledge_chunk(org_id, id = H(storeId, documentId, version, chunkNo), knowledge_store_id, knowledge_document_id, version_number, chunk_number, chunk_text (encryption at rest optional), title, token_count, embedding_model)`; `knowledge_chunk_metadata(org_id, id, knowledge_chunk_id, metadata_key, metadata_value)` with an index on `(org_id, metadata_key, metadata_value)`, written in the same transaction as its chunk, so metadata filters run on real columns and no query reads inside a JSON document.

### 2. Ingestion nodes (ordinary durable flows)
`orcon.doc.load@1` (PDF, DOCX, PPTX, XLSX/CSV, HTML with main-content extraction, Markdown, JSON, text; streamed from blobs; OCR only through a provider/connector), `orcon.text.split@1` (`CHARACTER`, `RECURSIVE_CHARACTER` default 1,000 chars / 200 overlap, `TOKEN` with embedded BPE tokenizers, `MARKDOWN`), `orcon.ai.embed@1`, `orcon.knowledge.upsert@1`, `orcon.knowledge.delete@1`. Upsert of an unchanged `contentHash` is a no-op; a new version inserts new chunks and deletes old ones in the same confirmation point. Builtin upsert is an internal effect (local transaction); external upsert is an effect with an `idempotencyKey` contract using the deterministic chunk id. Writes ≤ 512 chunks per transaction; embedding batches recorded (never re-embedded after a crash).

### 3. SQLite builtin backend
- Lexical: FTS5 external-content table `knowledge_chunk_fts5(title, chunk_text, content='knowledge_chunk', content_rowid='rowid', tokenize='unicode61 remove_diacritics 2')` kept in sync by insert/update/delete triggers (pattern of mneme `internal/db/migrations/001_initial.sql`; the triggers are listed in the database rulebook's exceptions file), ranked by `bm25()`; user queries are tokenized and each token quoted to neutralize FTS5 operators (pattern of mneme `store/search.go` `buildFTS5Query`/`ftsQuoteToken`); `porter unicode61` optional for English stores. Every query joins `knowledge_chunk` and filters `org_id` and `knowledge_store_id`.
- Vector: `knowledge_vector(org_id, id, knowledge_store_id, knowledge_chunk_id, embedding_model, dimension_count, embedding BLOB)` float32 little-endian, L2-normalized at write so cosine = dot product (pattern of mneme `store/embedding.go`). Search is brute force in Go with a bounded min-heap for top-k (not a full sort). A per-process cache holds decoded vectors per `(org, store, model)` — coherent because SQLite runs one process (ADR-0015 §5) — bounded (default 256 MiB, LRU by store), accounted in the worker memory budget (ADR-0016 §7), invalidated on write in the same process; without cache or above the cap, rows are streamed from the database. Extension point: `VectorIndex` interface so an in-memory HNSW can replace brute force later without API change.

### 4. PostgreSQL builtin backend
- Lexical: generated `tsvector` column with per-store `regconfig` (default `simple`) + GIN index; ranking `ts_rank_cd` over `websearch_to_tsquery`. This is not BM25; fusion uses ranks only.
- Vector: pgvector. Tables per dimension from a closed set (`knowledge_vector_384|512|768|1024|1536` as `vector(n)`, `knowledge_vector_3072` as `halfvec(3072)`), each with an HNSW index (`vector_cosine_ops` / `halfvec_cosine_ops`) and `(org_id, knowledge_store_id)` filters, using pgvector ≥ 0.8 iterative index scans to preserve recall under filters. Other dimensions: Matryoshka truncation when the model supports it, otherwise `knowledge_vector_any` without index (exact search). Without the pgvector extension, vectors are stored as `bytea` and searched with the same Go code as SQLite (warning surfaced). Forced RLS on all tables (ADR-0046); claims across organizations are not needed (retrieval is request-plane, single organization).

### 5. Hybrid retrieval (identical on both stores)
1. Metadata filters applied inside both channels, on `knowledge_chunk_metadata` (real columns; no JSON-path).
2. Lexical top-Nl (default 50), vector top-Nv (default 100, 2× over-fetch as in mneme `vectorSearchAll`).
3. Weighted Reciprocal Rank Fusion: `score(d) = Σ w_l / (60 + rank_l(d))`, deterministic tie-break by id (algorithm of mneme `internal/scoring/rrf.go`). Default weights lexical 1.0; vector 1.0 for provider embeddings, 0.8 for `tfidf-hash-v1` (mneme `weightVector`).
4. Optional reranker (`kind: RERANKER`), optional `multiQuery` (N recorded rewrites, all lists fused by RRF) and `compress`.
5. `topK` (default 4). Results recorded as nondeterminism with ids, scores and text references; provenance `EXTERNAL`.
Fusion runs in Go for both stores (one code path, one test suite).

### 6. Local embedder `tfidf-hash-v1`
Adapted from mneme `internal/embed/tfidf.go`: lowercase, split on whitespace/punctuation/symbols, whole words plus character n-grams of length 3–5, FNV-1a feature hashing into 512 buckets, L2 normalization. Changes: per-store IDF weights (bucket document frequencies maintained incrementally in `knowledge_store_document_frequency`; mneme uses term frequency only); unsigned bucket index (`h.Sum32() % uint32(dims)`, avoiding negative indexes on 32-bit platforms in mneme's `int(h.Sum32()) % dims`). Works offline; usable as primary embedder or as fallback vector channel when the provider is unavailable. Code adapted from mneme (Apache-2.0, same owner) — RRF, this embedder, the FTS5 and vector BLOB patterns, and the recency/importance formulas of ADR-0064 — carries attribution in Orcon's `NOTICE` file.

### 7. Model changes and reindexing
Chunks record `embedding_model`; vectors of different models are never compared. Changing a store's embedding sets the status `REINDEX_REQUIRED`. A person starts reindexing with the action `POST /knowledge-stores/{knowledgeStoreId}/reindex` (action verb `reindex`, ADR-0080 §2; permission `knowledge:manage`; audited; CLI `orcon knowledge reindex`), which returns `202` with a long-running operation (ADR-0053 §7) that writes new vectors before switching and moves the store through `REINDEXING` to `READY`. Calling it on a `READY` store recomputes the vectors anyway (for example after a chunking change); calling it on a `REINDEXING` store returns the operation already running.

### 8. External vector stores
`kind: VECTOR_STORE` connectors implement `Upsert`, `Delete`, `Query(vector, filter, k)`, optional `HybridQuery` and `LexicalQuery`: Pinecone, Qdrant, Weaviate, Redis, Supabase, Milvus, MongoDB Atlas, Chroma, Azure AI Search, external PGVector, Oracle, Zep. Native hybrid is used when offered; otherwise vector only (fused with lexical when available). One namespace/collection per store.

### 9. Isolation
Every query filters `org_id` and `knowledge_store_id`; caches are keyed by organization, store and model; stores are never shared across organizations.

## Consequences
Retrieval works offline on a single binary with SQLite and scales on PostgreSQL with the same fusion code; mneme's measured patterns are reused with known defects fixed; metadata filtering keeps foreign keys, checks and ordinary indexes.

## Alternatives considered
sqlite-vec or other SQLite vector extensions (rejected: require CGO or loadable extensions, breaking the pure-Go single binary); a mandatory external vector database (rejected by A3 and ADR-0004); fusion in SQL per store (rejected: two implementations of the same ranking); BM25 extensions for PostgreSQL under AGPL/SSPL (rejected by licence); filtering metadata with JSON-path operators (rejected by the Go rulebook DA-3 as amended and ADR-0077 §9 without an owner-approved exception); starting reindexing implicitly through a `PATCH` of the embedding configuration (rejected: the action a person wants is "reindex", and repeating a `PATCH` to relaunch a failed reindex is not discoverable).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-009 K19 (2026-10-13, owner decision).** §1 and §7 (`knowledge_store.backend`, `knowledge_store.status`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
