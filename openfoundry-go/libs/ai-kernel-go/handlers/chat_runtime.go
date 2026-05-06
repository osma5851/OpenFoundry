package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openfoundry/openfoundry-go/libs/ai-kernel-go/domain/evaluation"
	"github.com/openfoundry/openfoundry-go/libs/ai-kernel-go/domain/llm"
	"github.com/openfoundry/openfoundry-go/libs/ai-kernel-go/models"
)

// chat_runtime hosts the helpers that back chat completion / copilot
// ask / provider benchmark — the three endpoints that chain
// llm/runtime.CompleteText. All helpers are 1:1 with their Rust
// counterparts in libs/ai-kernel/src/handlers/chat.rs.

// loadProviderRows mirrors fn load_provider_rows. Returns every
// configured provider ordered by updated_at desc, created_at desc.
func loadProviderRows(ctx context.Context, pool *pgxpool.Pool) ([]models.LlmProvider, error) {
	rows, err := pool.Query(ctx,
		`SELECT `+providerColumns+` FROM ai_providers
          ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.LlmProvider, 0)
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// previewText mirrors fn preview_text — first `limit` runes of the
// trimmed content; appends "..." if truncated.
func previewText(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	runes := []rune(trimmed)
	if len(runes) > limit {
		return string(runes[:limit]) + "..."
	}
	return string(runes)
}

// attachmentContext mirrors fn attachment_context — formats
// attachments as "- <label>: …" lines for inclusion in the
// prompt-used echo.
func attachmentContext(attachments []models.ChatAttachment) string {
	if len(attachments) == 0 {
		return "none"
	}
	lines := make([]string, 0, len(attachments))
	for _, a := range attachments {
		label := "attachment"
		if a.Name != nil && strings.TrimSpace(*a.Name) != "" {
			label = *a.Name
		}
		switch a.Kind {
		case "image_url":
			url := "missing-url"
			if a.URL != nil {
				url = *a.URL
			}
			lines = append(lines, fmt.Sprintf("- %s: image url %s", label, url))
		case "image_base64":
			mime := "unknown"
			if a.MimeType != nil {
				mime = *a.MimeType
			}
			lines = append(lines, fmt.Sprintf("- %s: embedded %s image", label, mime))
		default:
			text := "text attachment"
			if a.Text != nil {
				text = *a.Text
			}
			lines = append(lines, fmt.Sprintf("- %s: %s", label, text))
		}
	}
	return strings.Join(lines, "\n")
}

// requiredModalities mirrors fn required_modalities — always includes
// "text"; appends "image" when any attachment kind starts with
// "image".
func requiredModalities(attachments []models.ChatAttachment) []string {
	out := []string{"text"}
	for _, a := range attachments {
		if strings.HasPrefix(a.Kind, "image") {
			out = append(out, "image")
			break
		}
	}
	return out
}

// modalityLabel mirrors fn modality_label.
func modalityLabel(required []string) string {
	for _, m := range required {
		if strings.EqualFold(m, "image") {
			return "image+text"
		}
	}
	return "text"
}

// privacyReason mirrors fn privacy_reason — returns the explicit
// "private network explicitly requested" when the body flag is set,
// or the PII-detected fallback when guardrail flagged a pii_* kind.
func privacyReason(verdict models.GuardrailVerdict, requirePrivateNetwork bool) *string {
	if requirePrivateNetwork {
		s := "private network explicitly requested"
		return &s
	}
	for _, f := range verdict.Flags {
		if strings.HasPrefix(f.Kind, "pii_") {
			s := "PII detected in prompt, preferring private-network providers"
			return &s
		}
	}
	return nil
}

// routingMetadata mirrors fn routing_metadata.
func routingMetadata(
	provider models.LlmProvider,
	requestedPrivateNetwork bool,
	privacyReason *string,
	candidates []models.LlmProvider,
	required []string,
) models.ChatRoutingMetadata {
	ids := make([]uuid.UUID, 0, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.ID)
	}
	return models.ChatRoutingMetadata{
		RequestedPrivateNetwork: requestedPrivateNetwork,
		UsedPrivateNetwork:      llm.ProviderUsesPrivateNetwork(provider),
		PrivacyReason:           privacyReason,
		CandidateProviderIDs:    ids,
		RequiredModalities:      append([]string{}, required...),
	}
}

// usageSummary mirrors fn usage_summary.
func usageSummary(provider models.LlmProvider, promptTokens, completionTokens, latencyMs int32, cacheHit bool) models.LlmUsageSummary {
	pt := promptTokens
	if pt < 0 {
		pt = 0
	}
	ct := completionTokens
	if ct < 0 {
		ct = 0
	}
	return models.LlmUsageSummary{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      pt + ct,
		EstimatedCostUSD: evaluation.EstimatedCostUSD(&provider, promptTokens, completionTokens, cacheHit),
		LatencyMs:        latencyMs,
		NetworkScope:     provider.RouteRules.NetworkScope,
		CacheHit:         cacheHit,
	}
}

// recordUsageEvent mirrors fn record_usage_event — best-effort insert
// into ai_llm_usage_events. Non-fatal at the call site (chat /
// benchmark return their replies even if the insert fails).
func recordUsageEvent(
	ctx context.Context,
	pool *pgxpool.Pool,
	providerID uuid.UUID,
	conversationID *uuid.UUID,
	requestKind, useCase, modality string,
	usage models.LlmUsageSummary,
	benchmarkGroupID *uuid.UUID,
	metadata any,
) error {
	id, err := uuid.NewV7()
	if err != nil {
		id = uuid.New()
	}
	metadataJSON, _ := json.Marshal(metadata)
	_, err = pool.Exec(ctx,
		`INSERT INTO ai_llm_usage_events (
            id, provider_id, conversation_id, request_kind, use_case,
            network_scope, modality, cache_hit, prompt_tokens,
            completion_tokens, total_tokens, estimated_cost_usd,
            latency_ms, benchmark_group_id, metadata
         ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		id, providerID, conversationID, requestKind, useCase,
		usage.NetworkScope, modality, usage.CacheHit, usage.PromptTokens,
		usage.CompletionTokens, usage.TotalTokens, usage.EstimatedCostUSD,
		usage.LatencyMs, benchmarkGroupID, metadataJSON,
	)
	return err
}

// BenchmarkProviders handles `POST /api/v1/providers/benchmark`.
// Mirrors fn benchmark_providers verbatim:
//   - validates prompt + guardrail (block sanitises 400 if blocked)
//   - loads providers, optionally filtered by body.provider_ids
//   - routes via gateway with privacy + modality filters
//   - calls llm.CompleteText for each routed provider, capturing
//     latency, tokens, error
//   - records ai_llm_usage_events per success
//   - scores quality/safety/latency/cost/overall, sorts desc,
//     picks the head as recommended_provider_id
func (h *ChatHandlers) BenchmarkProviders(w http.ResponseWriter, r *http.Request) {
	var body models.ProviderBenchmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.Prompt) == "" {
		writeError(w, http.StatusBadRequest, "benchmark prompt is required")
		return
	}

	promptVerdict := llm.EvaluateText(body.Prompt)
	if promptVerdict.Blocked {
		writeError(w, http.StatusBadRequest, "benchmark prompt is blocked by guardrails; sanitize it before benchmarking")
		return
	}

	ctx := r.Context()
	providers, err := loadProviderRows(ctx, h.Pool)
	if err != nil {
		dbError(w, err)
		return
	}
	if len(providers) == 0 {
		writeError(w, http.StatusNotFound, "no AI providers configured")
		return
	}

	candidates := providers
	if len(body.ProviderIDs) > 0 {
		filterSet := map[uuid.UUID]struct{}{}
		for _, id := range body.ProviderIDs {
			filterSet[id] = struct{}{}
		}
		filtered := make([]models.LlmProvider, 0, len(filterSet))
		for _, p := range providers {
			if _, ok := filterSet[p.ID]; ok {
				filtered = append(filtered, p)
			}
		}
		candidates = filtered
	}
	if len(candidates) == 0 {
		writeError(w, http.StatusNotFound, "no benchmark providers matched the requested ids")
		return
	}

	required := requiredModalities(body.Attachments)
	privacy := privacyReason(promptVerdict, body.RequirePrivateNetwork)
	routed := llm.RouteProviders(candidates, nil, body.UseCase, required, body.RequirePrivateNetwork, privacy != nil)
	if body.RequirePrivateNetwork && len(routed) == 0 {
		writeError(w, http.StatusBadRequest, "no private-network AI provider is configured for this benchmark")
		return
	}
	if len(routed) == 0 {
		writeError(w, http.StatusNotFound, "no eligible providers support this benchmark")
		return
	}

	benchmarkGroupID, err := uuid.NewV7()
	if err != nil {
		benchmarkGroupID = uuid.New()
	}

	systemPrompt := "You are an enterprise AI benchmark harness. Answer the user prompt clearly and concretely."
	if body.SystemPrompt != nil && strings.TrimSpace(*body.SystemPrompt) != "" {
		systemPrompt = *body.SystemPrompt
	}
	promptUsed := fmt.Sprintf("%s\n\nUser request: %s\n\nAttachments:\n%s",
		systemPrompt, promptVerdict.RedactedText, attachmentContext(body.Attachments))

	results := make([]models.ProviderBenchmarkResult, 0, len(routed))
	for _, provider := range routed {
		startedAt := time.Now()
		completion, completionErr := llm.CompleteText(ctx, nil, &provider,
			systemPrompt, body.Prompt, body.Attachments,
			0.2, body.MaxTokens)
		latencyMs := int32(time.Since(startedAt).Milliseconds())
		if latencyMs < 0 {
			latencyMs = 0
		}

		if completionErr != nil {
			errStr := completionErr.Error()
			results = append(results, models.ProviderBenchmarkResult{
				ProviderID:       provider.ID,
				ProviderName:     provider.Name,
				NetworkScope:     provider.RouteRules.NetworkScope,
				ReplyPreview:     "",
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
				EstimatedCostUSD: 0,
				LatencyMs:        latencyMs,
				CacheHit:         false,
				Guardrail:        models.DefaultGuardrailVerdict(),
				Score:            models.ProviderBenchmarkScore{},
				Error:            &errStr,
			})
			continue
		}

		promptTokens := completion.PromptTokens
		if est := llm.EstimateTokens(promptUsed); est > promptTokens {
			promptTokens = est
		}
		completionTokens := completion.CompletionTokens
		if est := llm.EstimateTokens(completion.Text); est > completionTokens {
			completionTokens = est
		}
		usage := usageSummary(provider, promptTokens, completionTokens, latencyMs, false)
		if completion.TotalTokens > usage.TotalTokens {
			usage.TotalTokens = completion.TotalTokens
		}

		replyVerdict := llm.EvaluateText(completion.Text)

		// Best-effort usage-event insert; ignore error.
		_ = recordUsageEvent(ctx, h.Pool, provider.ID, nil, "benchmark",
			body.UseCase, modalityLabel(required), usage, &benchmarkGroupID,
			map[string]any{
				"rubric_keywords": body.RubricKeywords,
				"provider_name":   provider.Name,
			})

		results = append(results, models.ProviderBenchmarkResult{
			ProviderID:       provider.ID,
			ProviderName:     provider.Name,
			NetworkScope:     usage.NetworkScope,
			ReplyPreview:     previewText(completion.Text, 280),
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			EstimatedCostUSD: usage.EstimatedCostUSD,
			LatencyMs:        usage.LatencyMs,
			CacheHit:         false,
			Guardrail:        replyVerdict,
			Score:            models.ProviderBenchmarkScore{},
		})
	}

	// Score successful results.
	successful := make([]int, 0, len(results))
	for i, r := range results {
		if r.Error == nil {
			successful = append(successful, i)
		}
	}

	minLatency, maxLatency := float32(0), float32(0)
	minCost, maxCost := float32(0), float32(0)
	if len(successful) > 0 {
		first := successful[0]
		minLatency = float32(results[first].LatencyMs)
		maxLatency = minLatency
		minCost = results[first].EstimatedCostUSD
		maxCost = minCost
		for _, idx := range successful[1:] {
			lat := float32(results[idx].LatencyMs)
			if lat < minLatency {
				minLatency = lat
			}
			if lat > maxLatency {
				maxLatency = lat
			}
			cost := results[idx].EstimatedCostUSD
			if cost < minCost {
				minCost = cost
			}
			if cost > maxCost {
				maxCost = cost
			}
		}
	}

	for _, idx := range successful {
		r := &results[idx]
		quality := evaluation.QualityScore(r.ReplyPreview, body.RubricKeywords)
		safety := evaluation.SafetyScore(&r.Guardrail)
		latency := evaluation.NormalizedScore(float32(r.LatencyMs), minLatency, maxLatency, true)
		cost := evaluation.NormalizedScore(r.EstimatedCostUSD, minCost, maxCost, true)
		r.Score = models.ProviderBenchmarkScore{
			Quality: quality,
			Latency: latency,
			Cost:    cost,
			Safety:  safety,
			Overall: evaluation.OverallBenchmarkScore(quality, safety, latency, cost),
		}
	}

	// Sort overall desc.
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score.Overall > results[j].Score.Overall
	})

	var recommended *uuid.UUID
	for _, r := range results {
		if r.Error == nil {
			id := r.ProviderID
			recommended = &id
			break
		}
	}

	writeJSON(w, http.StatusOK, models.ProviderBenchmarkResponse{
		BenchmarkGroupID:        benchmarkGroupID,
		UseCase:                 body.UseCase,
		PromptExcerpt:           summarizeTitle(body.Prompt),
		RequiredModalities:      required,
		RequestedPrivateNetwork: body.RequirePrivateNetwork,
		RecommendedProviderID:   recommended,
		Results:                 results,
		CreatedAt:               time.Now().UTC(),
	})
}
