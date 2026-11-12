// Copyright Envoy AI Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

package translator

import (
	"cmp"
	"fmt"
	"io"
	"path"
	"strconv"

	"github.com/tidwall/sjson"

	"github.com/envoyproxy/ai-gateway/internal/apischema/openai"
	"github.com/envoyproxy/ai-gateway/internal/internalapi"
	"github.com/envoyproxy/ai-gateway/internal/json"
	"github.com/envoyproxy/ai-gateway/internal/metrics"
	"github.com/envoyproxy/ai-gateway/internal/tracing/tracingapi"
)

// NewResponsesCompactOpenAIToOpenAITranslator creates a passthrough translator for
// /v1/responses/compact, defined by both the OpenAI Responses API and the
// Open Responses specification (https://www.openresponses.org).
func NewResponsesCompactOpenAIToOpenAITranslator(prefix string, modelNameOverride internalapi.ModelNameOverride) OpenAIResponsesCompactTranslator {
	return &responsesCompactToOpenAITranslator{
		modelNameOverride: modelNameOverride,
		path:              path.Join("/", prefix, "responses/compact"),
	}
}

type responsesCompactToOpenAITranslator struct {
	modelNameOverride internalapi.ModelNameOverride
	path              string
	// requestModel is the model sent upstream. The compact response carries no
	// model field, so it is reported as the response model.
	requestModel internalapi.RequestModel
}

// RequestBody implements [OpenAIResponsesCompactTranslator.RequestBody].
func (t *responsesCompactToOpenAITranslator) RequestBody(original []byte, req *openai.ResponseRequest, forceBodyMutation bool) (
	newHeaders []internalapi.Header, newBody []byte, err error,
) {
	if req != nil {
		t.requestModel = cmp.Or(t.modelNameOverride, req.Model)
	}
	if t.modelNameOverride != "" {
		newBody, err = sjson.SetBytesOptions(original, "model", t.modelNameOverride, sjsonOptions)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to set model name: %w", err)
		}
	}

	newBody = forceOriginalBodyIfEmpty(forceBodyMutation, newBody, original)

	newHeaders = []internalapi.Header{{pathHeaderName, t.path}}
	if len(newBody) > 0 {
		newHeaders = append(newHeaders, internalapi.Header{contentLengthHeaderName, strconv.Itoa(len(newBody))})
	}
	return
}

// ResponseHeaders implements [OpenAIResponsesCompactTranslator.ResponseHeaders].
func (t *responsesCompactToOpenAITranslator) ResponseHeaders(map[string]string) (
	newHeaders []internalapi.Header, err error,
) {
	return nil, nil
}

// ResponseBody implements [OpenAIResponsesCompactTranslator.ResponseBody].
//
// Compaction runs inference upstream, so the usage it reports is extracted for
// metrics, cost and rate limiting. The body itself is passed through unchanged.
func (t *responsesCompactToOpenAITranslator) ResponseBody(_ map[string]string, body io.Reader, _ bool, span tracingapi.ResponsesCompactSpan) (
	newHeaders []internalapi.Header, newBody []byte, tokenUsage metrics.TokenUsage, responseModel internalapi.ResponseModel, err error,
) {
	resp := &openai.ResponsesCompactResponse{}
	if err := json.NewDecoder(body).Decode(resp); err != nil {
		return nil, nil, tokenUsage, "", fmt.Errorf("failed to unmarshal body: %w", err)
	}
	setTokenUsageFromResponseUsage(&tokenUsage, resp.Usage)
	if span != nil {
		span.RecordResponse(resp)
	}
	return nil, nil, tokenUsage, t.requestModel, nil
}

// ResponseError implements [OpenAIResponsesCompactTranslator.ResponseError].
func (t *responsesCompactToOpenAITranslator) ResponseError(respHeaders map[string]string, body io.Reader) (
	newHeaders []internalapi.Header, mutatedBody []byte, err error,
) {
	return convertErrorOpenAIToOpenAIError(respHeaders, body)
}
