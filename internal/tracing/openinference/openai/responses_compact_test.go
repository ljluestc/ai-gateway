// Copyright Envoy AI Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/envoyproxy/ai-gateway/internal/apischema/openai"
	"github.com/envoyproxy/ai-gateway/internal/testing/testotel"
	"github.com/envoyproxy/ai-gateway/internal/tracing/openinference"
)

var (
	basicCompactReq     = &openai.ResponseRequest{Model: openai.ModelGPT5Nano, Input: openai.ResponseNewParamsInputUnion{OfString: ptr("hello")}}
	basicCompactReqBody = mustJSON(basicCompactReq)
	basicCompactResp    = &openai.ResponsesCompactResponse{
		ID:        "resp_compact_1",
		Object:    "response.compaction",
		CreatedAt: 1764967971,
		Output:    []openai.ResponseOutputItemUnion{},
		Usage: &openai.ResponseUsage{
			InputTokens:         100,
			InputTokensDetails:  openai.ResponseUsageInputTokensDetails{CachedTokens: 10},
			OutputTokens:        20,
			OutputTokensDetails: openai.ResponseUsageOutputTokensDetails{ReasoningTokens: 3},
			TotalTokens:         120,
		},
	}
)

func TestResponsesCompactRecorder_StartParams(t *testing.T) {
	recorder := NewResponsesCompactRecorderFromEnv()

	spanName, opts := recorder.StartParams(basicCompactReq, basicCompactReqBody)
	actualSpan := testotel.RecordNewSpan(t, spanName, opts...)

	require.Equal(t, "ResponsesCompact", actualSpan.Name)
	require.Equal(t, oteltrace.SpanKindInternal, actualSpan.SpanKind)
}

func TestResponsesCompactRecorder_RecordRequest(t *testing.T) {
	for _, tt := range []struct {
		name          string
		config        *openinference.TraceConfig
		expectedAttrs []attribute.KeyValue
	}{
		{
			name:   "basic request",
			config: &openinference.TraceConfig{},
			expectedAttrs: []attribute.KeyValue{
				attribute.String(openinference.SpanKind, openinference.SpanKindLLM),
				attribute.String(openinference.LLMSystem, openinference.LLMSystemOpenAI),
				attribute.String(openinference.LLMModelName, openai.ModelGPT5Nano),
				attribute.String(openinference.InputValue, string(basicCompactReqBody)),
				attribute.String(openinference.InputMimeType, openinference.MimeTypeJSON),
			},
		},
		{
			name:   "hide inputs",
			config: &openinference.TraceConfig{HideInputs: true},
			expectedAttrs: []attribute.KeyValue{
				attribute.String(openinference.SpanKind, openinference.SpanKindLLM),
				attribute.String(openinference.LLMSystem, openinference.LLMSystemOpenAI),
				attribute.String(openinference.LLMModelName, openai.ModelGPT5Nano),
				attribute.String(openinference.InputValue, openinference.RedactedValue),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := NewResponsesCompactRecorder(tt.config)
			actualSpan := testotel.RecordWithSpan(t, func(span oteltrace.Span) bool {
				recorder.RecordRequest(span, basicCompactReq, basicCompactReqBody)
				return false
			})
			openinference.RequireAttributesEqual(t, tt.expectedAttrs, actualSpan.Attributes)
			require.Equal(t, trace.Status{Code: codes.Unset}, actualSpan.Status)
		})
	}
}

func TestResponsesCompactRecorder_RecordResponse(t *testing.T) {
	usageAttrs := []attribute.KeyValue{
		attribute.Int(openinference.LLMTokenCountPrompt, 100),
		attribute.Int(openinference.LLMTokenCountCompletion, 20),
		attribute.Int(openinference.LLMTokenCountTotal, 120),
		attribute.Int(openinference.LLMTokenCountPromptCacheHit, 10),
		attribute.Int(openinference.LLMTokenCountCompletionReasoning, 3),
	}
	for _, tt := range []struct {
		name          string
		config        *openinference.TraceConfig
		expectedAttrs []attribute.KeyValue
	}{
		{
			name:   "basic response",
			config: &openinference.TraceConfig{},
			expectedAttrs: append(append([]attribute.KeyValue{}, usageAttrs...),
				attribute.String(openinference.OutputMimeType, openinference.MimeTypeJSON),
				attribute.String(openinference.OutputValue, string(mustJSON(basicCompactResp))),
			),
		},
		{
			name:   "hide outputs",
			config: &openinference.TraceConfig{HideOutputs: true},
			expectedAttrs: append(append([]attribute.KeyValue{}, usageAttrs...),
				attribute.String(openinference.OutputValue, openinference.RedactedValue),
			),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := NewResponsesCompactRecorder(tt.config)
			actualSpan := testotel.RecordWithSpan(t, func(span oteltrace.Span) bool {
				recorder.RecordResponse(span, basicCompactResp)
				return false
			})
			openinference.RequireAttributesEqual(t, tt.expectedAttrs, actualSpan.Attributes)
			require.Equal(t, trace.Status{Code: codes.Ok}, actualSpan.Status)
		})
	}
}

func TestResponsesCompactRecorder_RecordResponseOnError(t *testing.T) {
	recorder := NewResponsesCompactRecorderFromEnv()
	actualSpan := testotel.RecordWithSpan(t, func(span oteltrace.Span) bool {
		recorder.RecordResponseOnError(span, 400, []byte(`{"error":{"message":"bad"}}`))
		return false
	})
	require.Equal(t, codes.Error, actualSpan.Status.Code)
	require.Len(t, actualSpan.Events, 1)
	require.Equal(t, "exception", actualSpan.Events[0].Name)
}

func TestResponsesCompactRecorder_RecordResponseChunks(t *testing.T) {
	recorder := NewResponsesCompactRecorder(nil)
	_, ok := recorder.(*ResponsesCompactRecorder)
	require.True(t, ok)
	actualSpan := testotel.RecordWithSpan(t, func(span oteltrace.Span) bool {
		recorder.RecordResponseChunks(span, []*struct{}{})
		return false
	})
	openinference.RequireEventsEqual(t, []trace.Event{}, actualSpan.Events)
}
