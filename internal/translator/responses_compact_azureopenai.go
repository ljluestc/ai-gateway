// Copyright Envoy AI Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

package translator

import (
	"github.com/envoyproxy/ai-gateway/internal/apischema/openai"
	"github.com/envoyproxy/ai-gateway/internal/internalapi"
)

// NewResponsesCompactOpenAIToAzureOpenAITranslator implements [Factory] for OpenAI to Azure OpenAI translation
// for /v1/responses/compact.
func NewResponsesCompactOpenAIToAzureOpenAITranslator(apiVersion string, modelNameOverride internalapi.ModelNameOverride) OpenAIResponsesCompactTranslator {
	return &openAIToAzureOpenAITranslatorV1ResponsesCompact{
		apiVersion: apiVersion,
		responsesCompactToOpenAITranslator: responsesCompactToOpenAITranslator{
			modelNameOverride: modelNameOverride,
			path:              "/openai/responses/compact",
		},
	}
}

type openAIToAzureOpenAITranslatorV1ResponsesCompact struct {
	apiVersion string
	responsesCompactToOpenAITranslator
}

// RequestBody implements [OpenAIResponsesCompactTranslator.RequestBody].
func (o *openAIToAzureOpenAITranslatorV1ResponsesCompact) RequestBody(raw []byte, req *openai.ResponseRequest, forceBodyMutation bool) (
	newHeaders []internalapi.Header, newBody []byte, err error,
) {
	newHeaders, newBody, err = o.responsesCompactToOpenAITranslator.RequestBody(raw, req, forceBodyMutation)
	if err != nil {
		return nil, nil, err
	}
	newHeaders[0] = internalapi.Header{pathHeaderName, appendAzureOpenAIAPIVersion(newHeaders[0].Value(), o.apiVersion)}
	return
}
