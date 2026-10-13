// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"cmp"
	"slices"
)

// Statuses of an AI mapping (docs/reference/n8n-ai-coverage.md).
const (
	// AISupported is row S of SPEC-006 Annex B.
	AISupported = "SUPPORTED"
	// AIAdapted is row A: mapped with a difference the import reports.
	AIAdapted = "ADAPTED"
)

// aiPlan is the AI mapping that SPEC-006 Annex B commits to, for the n8n AI
// types that appear in the measurement corpus. It is planning data: the
// coverage of a catalog that maps AI is "planned" until the AI importer of
// SPEC-006 (AI16) replaces this table with the real mapping. A type the annex
// does not name, and LangChain Code (row U), are not in it, so they count as
// unmapped and are listed in the report.
//
// Keys are the type names without the package prefix.
var aiPlan = map[string]string{
	// Root nodes: agents and chains.
	"agent": AIAdapted, "chainLlm": AISupported, "chainRetrievalQa": AISupported,
	"chainSummarization": AISupported, "informationExtractor": AISupported,
	"textClassifier": AISupported, "sentimentAnalysis": AISupported, "mcpClientTool": AISupported,
	"chat": AISupported, "openAi": AIAdapted, "googleGemini": AIAdapted,
	// Triggers.
	"chatTrigger": AISupported, "mcpTrigger": AISupported,
	// Chat models, language models and embeddings.
	"lmChatAnthropic": AISupported, "lmChatOpenAi": AISupported, "lmChatGoogleGemini": AISupported,
	"lmChatAzureOpenAi": AISupported, "lmChatMistralCloud": AISupported, "lmChatGroq": AISupported,
	"lmChatDeepSeek": AISupported, "lmChatOllama": AISupported, "lmChatOpenRouter": AISupported,
	"lmChatXAiGrok": AISupported, "lmOllama": AIAdapted, "lmOpenHuggingFaceInference": AIAdapted,
	"embeddingsOpenAi": AISupported, "embeddingsGoogleGemini": AISupported,
	"embeddingsMistralCloud": AISupported, "embeddingsOllama": AISupported,
	"rerankerCohere": AISupported,
	// Memory.
	"memoryBufferWindow": AIAdapted, "memoryManager": AISupported, "memoryPostgresChat": AISupported,
	"memoryRedisChat": AISupported, "memoryMongoDbChat": AISupported,
	// Vector stores, loaders, splitters, retrievers and parsers.
	"vectorStoreInMemory": AIAdapted, "vectorStorePGVector": AISupported, "vectorStorePinecone": AISupported,
	"vectorStoreQdrant": AISupported, "vectorStoreSupabase": AISupported, "vectorStoreWeaviate": AISupported,
	"vectorStoreMongoDBAtlas": AISupported, "documentDefaultDataLoader": AISupported,
	"textSplitterCharacterTextSplitter": AISupported, "textSplitterRecursiveCharacterTextSplitter": AISupported,
	"textSplitterTokenSplitter": AISupported, "retrieverVectorStore": AISupported, "retrieverWorkflow": AISupported,
	"outputParserStructured": AISupported, "outputParserAutofixing": AISupported, "outputParserItemList": AISupported,
	// Tools.
	"toolWorkflow": AISupported, "toolHttpRequest": AISupported, "toolCode": AISupported,
	"toolCalculator": AISupported, "toolThink": AISupported, "toolVectorStore": AISupported,
	"toolWikipedia": AISupported, "toolSerpApi": AISupported,
}

// PlannedAI returns the AI entries of the planned catalog: the types of
// SPEC-006 Annex B (rows S and A), the Tool variants of built-in nodes and the
// $fromAI() function.
func PlannedAI() []AIEntry {
	out := []AIEntry{
		{Type: AIToolVariants, Status: AISupported},
		{Type: AIFromAI, Status: AISupported},
		{Type: basePrefix + "openAi", Status: AIAdapted},
	}
	for name, status := range aiPlan {
		out = append(out, AIEntry{Type: aiPrefix + name, Status: status})
	}
	slices.SortFunc(out, func(a, b AIEntry) int { return cmp.Compare(a.Type, b.Type) })
	return out
}

// PlannedBase is the starting point of a planning run: the core nodes (always
// present) and the AI mapping SPEC-006 Annex B commits to, with no connector.
func PlannedBase() *Catalog { return (&Catalog{}).WithAI(PlannedAI()...) }

// Installed is the catalog of the release: the core nodes, the packages the
// binary registers (read through their n8n.types[] and parameterMap,
// ADR-0034 section 8) and the AI nodes the binary maps. This build registers
// no official package and has no AI importer, so the catalog holds the core
// nodes only. The connector registry of the manifest cut (CN1) and the AI
// importer of SPEC-006 (AI16) extend this function.
func Installed() *Catalog { return &Catalog{} }
