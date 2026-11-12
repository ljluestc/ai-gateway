// Copyright Envoy AI Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

package translator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/envoyproxy/ai-gateway/internal/apischema/openai"
	"github.com/envoyproxy/ai-gateway/internal/json"
	"github.com/envoyproxy/ai-gateway/internal/tracing/tracingapi"
)

func TestResponsesCompactOpenAIToOpenAI_RequestBody(t *testing.T) {
	original := []byte(`{"model":"gpt-5","input":[{"type":"message","role":"assistant","phase":"final_answer","content":"hi"}]}`)
	for _, tc := range []struct {
		name              string
		prefix            string
		forceBodyMutation bool
		modelNameOverride string
		expPath           string
		expBody           []byte
	}{
		{name: "no mutation", expPath: "/responses/compact"},
		{name: "with prefix", prefix: "/openai", expPath: "/openai/responses/compact"},
		{
			name:              "model override",
			modelNameOverride: "gpt-5-2025-08-07",
			expPath:           "/responses/compact",
			expBody:           []byte(`{"model":"gpt-5-2025-08-07","input":[{"type":"message","role":"assistant","phase":"final_answer","content":"hi"}]}`),
		},
		{name: "force body mutation", forceBodyMutation: true, expPath: "/responses/compact", expBody: original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewResponsesCompactOpenAIToOpenAITranslator(tc.prefix, tc.modelNameOverride)
			headers, body, err := tr.RequestBody(original, &openai.ResponseRequest{Model: "gpt-5"}, tc.forceBodyMutation)
			require.NoError(t, err)
			require.Equal(t, pathHeaderName, headers[0].Key())
			assert.Equal(t, tc.expPath, headers[0].Value())
			// The upstream body must keep Open Responses fields such as "phase" byte-for-byte.
			require.Equal(t, string(tc.expBody), string(body))
			if len(tc.expBody) > 0 {
				require.Len(t, headers, 2)
				require.Equal(t, contentLengthHeaderName, headers[1].Key())
			} else {
				require.Len(t, headers, 1)
			}
		})
	}
}

func TestResponsesCompactOpenAIToOpenAI_ResponseBody(t *testing.T) {
	const respBody = `{
  "id": "resp_compact_1",
  "object": "response.compaction",
  "created_at": 1764967971,
  "output": [
    {"id": "msg_1", "type": "message", "status": "completed", "role": "assistant", "phase": "final_answer",
     "content": [{"type": "output_text", "text": "summary", "annotations": []}]},
    {"id": "cmp_1", "type": "compaction", "encrypted_content": "abc"}
  ],
  "usage": {
    "input_tokens": 120,
    "input_tokens_details": {"cached_tokens": 20},
    "output_tokens": 30,
    "output_tokens_details": {"reasoning_tokens": 5},
    "total_tokens": 150
  }
}`

	t.Run("usage and model", func(t *testing.T) {
		tr := NewResponsesCompactOpenAIToOpenAITranslator("", "gpt-5-2025-08-07")
		_, _, err := tr.RequestBody([]byte(`{"model":"gpt-5"}`), &openai.ResponseRequest{Model: "gpt-5"}, false)
		require.NoError(t, err)

		span := &recordingCompactSpan{}
		headers, body, usage, model, err := tr.ResponseBody(nil, strings.NewReader(respBody), true, span)
		require.NoError(t, err)
		require.Nil(t, headers)
		require.Nil(t, body, "response body must pass through unchanged")
		// The compact response has no model field, so the model sent upstream is reported.
		require.Equal(t, "gpt-5-2025-08-07", model)

		in, _ := usage.InputTokens()
		out, _ := usage.OutputTokens()
		total, _ := usage.TotalTokens()
		cached, _ := usage.CachedInputTokens()
		reasoning, _ := usage.ReasoningTokens()
		require.Equal(t, []uint32{120, 30, 150, 20, 5}, []uint32{in, out, total, cached, reasoning})

		require.NotNil(t, span.resp)
		require.Equal(t, "response.compaction", span.resp.Object)
		require.Len(t, span.resp.Output, 2)
		require.Equal(t, "final_answer", span.resp.Output[0].OfOutputMessage.Phase)
		require.Equal(t, "abc", span.resp.Output[1].OfCompaction.EncryptedContent)
	})

	t.Run("request model without override", func(t *testing.T) {
		tr := NewResponsesCompactOpenAIToOpenAITranslator("", "")
		_, _, err := tr.RequestBody([]byte(`{"model":"gpt-5"}`), &openai.ResponseRequest{Model: "gpt-5"}, false)
		require.NoError(t, err)
		_, _, _, model, err := tr.ResponseBody(nil, strings.NewReader(respBody), true, nil)
		require.NoError(t, err)
		require.Equal(t, "gpt-5", model)
	})

	t.Run("missing usage", func(t *testing.T) {
		tr := NewResponsesCompactOpenAIToOpenAITranslator("", "")
		_, _, usage, _, err := tr.ResponseBody(nil, strings.NewReader(`{"id":"r","object":"response.compaction","output":[],"created_at":1}`), true, nil)
		require.NoError(t, err)
		_, ok := usage.InputTokens()
		require.False(t, ok)
	})

	t.Run("invalid json", func(t *testing.T) {
		tr := NewResponsesCompactOpenAIToOpenAITranslator("", "")
		_, _, _, _, err := tr.ResponseBody(nil, strings.NewReader(`{`), true, nil)
		require.ErrorContains(t, err, "failed to unmarshal body")
	})
}

func TestResponsesCompactOpenAIToOpenAI_ResponseHeaders(t *testing.T) {
	tr := NewResponsesCompactOpenAIToOpenAITranslator("", "")
	headers, err := tr.ResponseHeaders(map[string]string{"content-type": "application/json"})
	require.NoError(t, err)
	require.Nil(t, headers)
}

func TestResponsesCompactOpenAIToOpenAI_ResponseError(t *testing.T) {
	tr := NewResponsesCompactOpenAIToOpenAITranslator("", "")
	_, body, err := tr.ResponseError(
		map[string]string{statusHeaderName: "503", contentTypeHeaderName: "text/plain"},
		strings.NewReader("service unavailable"),
	)
	require.NoError(t, err)
	var got openai.Error
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "service unavailable", got.Error.Message)
}

func TestResponsesCompactOpenAIToAzure_RequestBody(t *testing.T) {
	original := []byte(`{"model":"gpt-5","input":"hello"}`)

	tr := NewResponsesCompactOpenAIToAzureOpenAITranslator("2025-04-01-preview", "")
	headers, body, err := tr.RequestBody(original, &openai.ResponseRequest{Model: "gpt-5"}, false)
	require.NoError(t, err)
	require.Nil(t, body)
	require.Equal(t, "/openai/responses/compact?api-version=2025-04-01-preview", headers[0].Value())

	tr = NewResponsesCompactOpenAIToAzureOpenAITranslator("2025-04-01-preview", "my-deployment")
	headers, body, err = tr.RequestBody(original, &openai.ResponseRequest{Model: "gpt-5"}, false)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"my-deployment","input":"hello"}`, string(body))
	require.Equal(t, "/openai/responses/compact?api-version=2025-04-01-preview", headers[0].Value())

	_, _, _, model, err := tr.ResponseBody(nil, strings.NewReader(`{"id":"r","object":"response.compaction","output":[],"created_at":1}`), true, nil)
	require.NoError(t, err)
	require.Equal(t, "my-deployment", model)
}

// recordingCompactSpan captures the response recorded on a compact span.
type recordingCompactSpan struct {
	resp *openai.ResponsesCompactResponse
}

var _ tracingapi.ResponsesCompactSpan = (*recordingCompactSpan)(nil)

func (s *recordingCompactSpan) RecordResponse(resp *openai.ResponsesCompactResponse) { s.resp = resp }
func (s *recordingCompactSpan) RecordResponseChunk(*struct{})                        {}
func (s *recordingCompactSpan) EndSpanOnError(int, []byte)                           {}
func (s *recordingCompactSpan) EndSpan()                                             {}
