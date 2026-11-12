// Copyright Envoy AI Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

package openai

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	openaiSchema "github.com/envoyproxy/ai-gateway/internal/apischema/openai"
	"github.com/envoyproxy/ai-gateway/internal/json"
	"github.com/envoyproxy/ai-gateway/internal/tracing/openinference"
	"github.com/envoyproxy/ai-gateway/internal/tracing/tracingapi"
)

// ResponsesCompactRecorder implements recorders for OpenInference /v1/responses/compact spans.
type ResponsesCompactRecorder struct {
	traceConfig *openinference.TraceConfig
}

// NewResponsesCompactRecorderFromEnv creates a tracingapi.ResponsesCompactRecorder
// from environment variables using the OpenInference configuration specification.
func NewResponsesCompactRecorderFromEnv() tracingapi.ResponsesCompactRecorder {
	return NewResponsesCompactRecorder(nil)
}

// NewResponsesCompactRecorder creates a tracingapi.ResponsesCompactRecorder with the
// given config using the OpenInference configuration specification.
func NewResponsesCompactRecorder(config *openinference.TraceConfig) tracingapi.ResponsesCompactRecorder {
	if config == nil {
		config = openinference.NewTraceConfigFromEnv()
	}
	return &ResponsesCompactRecorder{traceConfig: config}
}

// StartParams implements the same method as defined in tracingapi.ResponsesCompactRecorder.
func (r *ResponsesCompactRecorder) StartParams(*openaiSchema.ResponseRequest, []byte) (spanName string, opts []trace.SpanStartOption) {
	return "ResponsesCompact", startOpts
}

// RecordRequest implements the same method as defined in tracingapi.ResponsesCompactRecorder.
//
// Compaction invokes the model to summarize the conversation, so the span is an LLM span.
func (r *ResponsesCompactRecorder) RecordRequest(span trace.Span, req *openaiSchema.ResponseRequest, body []byte) {
	attrs := []attribute.KeyValue{
		attribute.String(openinference.SpanKind, openinference.SpanKindLLM),
		attribute.String(openinference.LLMSystem, openinference.LLMSystemOpenAI),
		attribute.String(openinference.LLMModelName, req.Model),
	}
	if r.traceConfig.HideInputs {
		attrs = append(attrs, attribute.String(openinference.InputValue, openinference.RedactedValue))
	} else {
		attrs = append(attrs,
			attribute.String(openinference.InputValue, string(body)),
			attribute.String(openinference.InputMimeType, openinference.MimeTypeJSON),
		)
	}
	span.SetAttributes(attrs...)
}

// RecordResponse implements the same method as defined in tracingapi.ResponsesCompactRecorder.
func (r *ResponsesCompactRecorder) RecordResponse(span trace.Span, resp *openaiSchema.ResponsesCompactResponse) {
	attrs := responseUsageAttrs(resp.Usage)
	bodyString := openinference.RedactedValue
	if !r.traceConfig.HideOutputs {
		if marshaled, err := json.Marshal(resp); err == nil {
			bodyString = string(marshaled)
			attrs = append(attrs, attribute.String(openinference.OutputMimeType, openinference.MimeTypeJSON))
		}
	}
	attrs = append(attrs, attribute.String(openinference.OutputValue, bodyString))
	span.SetAttributes(attrs...)
	span.SetStatus(codes.Ok, "")
}

// RecordResponseOnError implements the same method as defined in tracingapi.ResponsesCompactRecorder.
func (r *ResponsesCompactRecorder) RecordResponseOnError(span trace.Span, statusCode int, body []byte) {
	openinference.RecordResponseError(span, statusCode, string(body))
}

// RecordResponseChunks implements SpanRecorder.RecordResponseChunks as a no-op (responses/compact doesn't stream).
func (r *ResponsesCompactRecorder) RecordResponseChunks(trace.Span, []*struct{}) {}
