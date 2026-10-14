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
	// AIUnsupported is row U: the annex says the type has no counterpart.
	AIUnsupported = "UNSUPPORTED"
)

// aiPlan classifies the AI types of the pinned n8n release by the rows of
// SPEC-006 Annex B (docs/reference/n8n-ai-coverage.md), family rows included:
// the chat model, language model, embedding, memory, vector store, loader,
// splitter, retriever, parser and tool families map as their row says (S unless
// the row says A). It is planning data: the AI importer of SPEC-006 (AI16)
// replaces it with the real mapping. A type here that the annex marks U is
// unsupported; a type of the inventory that is neither here nor in aiUnnamed
// fails TestAIPlanCoversInventory, so a new release cannot change the figures
// unnoticed.
//
// Keys are full n8n types; the LangChain package prefix is added by lc.
var aiPlan = buildAIPlan()

func buildAIPlan() map[string]string {
	plan := map[string]string{}
	lc := func(status string, names ...string) {
		for _, n := range names {
			plan[aiPrefix+n] = status
		}
	}
	// Root nodes: agents and chains.
	lc(AISupported, "chainLlm", "chainRetrievalQa", "chainSummarization", "informationExtractor",
		"textClassifier", "sentimentAnalysis", "modelSelector", "mcpClient", "chat")
	lc(AIAdapted, "agent", "openAi", "googleGemini")
	lc(AIUnsupported, "code") // LangChain Code: row U
	// Triggers.
	lc(AISupported, "chatTrigger", "mcpTrigger")
	// Chat models, language models and embeddings (family rows).
	lc(AISupported, "lmChatAnthropic", "lmChatOpenAi", "lmChatGoogleGemini", "lmChatGoogleVertex",
		"lmChatAzureOpenAi", "lmChatAwsBedrock", "lmChatMistralCloud", "lmChatGroq", "lmChatDeepSeek",
		"lmChatOllama", "lmChatOpenRouter", "lmChatVercelAiGateway", "lmChatXAiGrok")
	lc(AIAdapted, "lmOllama", "lmOpenHuggingFaceInference", "lmCohere")
	lc(AISupported, "embeddingsOpenAi", "embeddingsGoogleGemini", "embeddingsGoogleVertex",
		"embeddingsAzureOpenAi", "embeddingsAwsBedrock", "embeddingsMistralCloud", "embeddingsOllama",
		"embeddingsCohere", "embeddingsHuggingFaceInference", "rerankerCohere")
	// Memory.
	lc(AISupported, "memoryManager", "memoryPostgresChat", "memoryRedisChat", "memoryMongoDbChat",
		"memoryXata", "memoryZep", "memoryMotorhead")
	lc(AIAdapted, "memoryBufferWindow")
	// Vector stores, loaders, splitters, retrievers and parsers.
	lc(AISupported, "vectorStorePGVector", "vectorStorePinecone", "vectorStoreQdrant", "vectorStoreSupabase",
		"vectorStoreWeaviate", "vectorStoreMongoDBAtlas", "vectorStoreRedis", "vectorStoreMilvus", "vectorStoreZep",
		"documentDefaultDataLoader", "textSplitterCharacterTextSplitter", "textSplitterRecursiveCharacterTextSplitter",
		"textSplitterTokenSplitter", "retrieverVectorStore", "retrieverWorkflow", "retrieverMultiQuery",
		"retrieverContextualCompression", "outputParserStructured", "outputParserAutofixing", "outputParserItemList")
	lc(AIAdapted, "vectorStoreInMemory", "documentGithubLoader")
	// Tools.
	lc(AISupported, "agentTool", "toolWorkflow", "toolHttpRequest", "toolCode", "toolCalculator", "toolThink",
		"toolVectorStore", "toolWikipedia", "toolSerpApi", "toolSearXng", "toolWolframAlpha", "mcpClientTool")
	// Nodes of the base package that the annex names as AI nodes.
	plan[basePrefix+"aiTransform"] = AIAdapted // AI Transform -> orcon.code@1, the generated code is kept
	plan[basePrefix+"openAi"] = AIAdapted      // the legacy OpenAI application node
	return plan
}

// aiUnnamed lists the AI types of the pinned release that SPEC-006 Annex B does
// not name. They count as unmapped and are reported; naming them is an
// amendment of that annex, not a change here.
var aiUnnamed = []string{
	aiPrefix + "manualChatTrigger",
	aiPrefix + "lmOpenAi",
	aiPrefix + "documentBinaryInputLoader",
	aiPrefix + "documentJsonInputLoader",
	aiPrefix + "memoryChatRetriever",
}

// AIPlanTypes returns every type the plan classifies, sorted: the mapped ones,
// the explicitly unsupported ones and the unnamed ones.
func AIPlanTypes() []string {
	out := make([]string, 0, len(aiPlan)+len(aiUnnamed))
	for t := range aiPlan {
		out = append(out, t)
	}
	out = append(out, aiUnnamed...)
	slices.Sort(out)
	return out
}

// AIClassified reports whether the plan says something about the AI type.
func AIClassified(nodeType string) bool {
	if _, ok := aiPlan[nodeType]; ok {
		return true
	}
	return slices.Contains(aiUnnamed, nodeType)
}

// PlannedAI returns the AI entries of the planned catalog: the types of
// SPEC-006 Annex B marked S or A, the Tool variants of built-in nodes and the
// $fromAI() function. Unsupported and unnamed types are not in it, so they
// count as unmapped.
func PlannedAI() []AIEntry {
	out := []AIEntry{
		{Type: AIToolVariants, Status: AISupported},
		{Type: AIFromAI, Status: AISupported},
	}
	for t, status := range aiPlan {
		if status != AIUnsupported {
			out = append(out, AIEntry{Type: t, Status: status})
		}
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
