package chat_completions

import (
	"bytes"
	"encoding/json"
)

type chatReqInput struct {
	ReasoningEffort string          `json:"reasoning_effort"`
	Messages        []chatMessage   `json:"messages"`
	Tools           []chatTool      `json:"tools"`
	ToolChoice      json.RawMessage `json:"tool_choice"`
	ResponseFormat  *chatRespFormat `json:"response_format"`
	Text            *chatTextCfg    `json:"text"`
}

type chatMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCalls  []chatToolCall  `json:"tool_calls"`
	ToolCallID string          `json:"tool_call_id"`
}

type chatTool struct {
	Type     string          `json:"type"`
	Function *chatToolFunc   `json:"function"`
	Raw      json.RawMessage `json:"-"`
}

func (t *chatTool) UnmarshalJSON(data []byte) error {
	type alias chatTool
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*t = chatTool(a)
	t.Raw = append(t.Raw[:0], data...)
	return nil
}

type chatToolFunc struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      *bool           `json:"strict"`
}

type chatToolCall struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatRespFormat struct {
	Type       string          `json:"type"`
	JSONSchema *chatJSONSchema `json:"json_schema"`
}

type chatJSONSchema struct {
	Name   string          `json:"name"`
	Strict *bool           `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type chatTextCfg struct {
	Verbosity json.RawMessage `json:"verbosity"`
}

type chatContentPart struct {
	Type     string               `json:"type"`
	Text     string               `json:"text"`
	ImageURL *chatContentImageURL `json:"image_url"`
	File     *chatContentFile     `json:"file"`
}

type chatContentImageURL struct {
	URL string `json:"url"`
}

type chatContentFile struct {
	FileData string `json:"file_data"`
	Filename string `json:"filename"`
}

// ConvertOpenAIRequestToCodex converts an OpenAI Chat Completions request JSON
// into an OpenAI Responses API request JSON using a single unmarshal/build/
// marshal pipeline while preserving the current mainline compatibility
// behaviors.
func ConvertOpenAIRequestToCodex(modelName string, inputRawJSON []byte, stream bool) []byte {
	req, ok := cachedOpenAIRequest(inputRawJSON)
	if !ok {
		if err := json.Unmarshal(inputRawJSON, &req); err == nil && len(inputRawJSON) > 0 {
			openAIRequestCache.put(requestCacheKey(inputRawJSON), req)
		}
	}

	var funcNames []string
	for _, tool := range req.Tools {
		if tool.Type == "function" && tool.Function != nil {
			funcNames = append(funcNames, tool.Function.Name)
		}
	}
	originalToolNameMap := buildShortNameMap(funcNames)

	effort := req.ReasoningEffort
	if effort == "" {
		effort = "medium"
	}

	out := map[string]any{
		"instructions":        "",
		"stream":              stream,
		"parallel_tool_calls": true,
		"include":             []string{"reasoning.encrypted_content"},
		"model":               modelName,
		"store":               false,
		"reasoning": map[string]any{
			"effort":  effort,
			"summary": "auto",
		},
	}

	input := make([]any, 0, len(req.Messages))
	for _, message := range req.Messages {
		switch message.Role {
		case "tool":
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": message.ToolCallID,
				"output":  rawToString(message.Content),
			})
		default:
			role := message.Role
			if role == "system" {
				role = "developer"
			}

			contentParts := buildContentParts(message.Role, message.Content)
			if message.Role != "assistant" || len(contentParts) > 0 {
				input = append(input, map[string]any{
					"type":    "message",
					"role":    role,
					"content": contentParts,
				})
			}

			if message.Role == "assistant" {
				for _, toolCall := range message.ToolCalls {
					if toolCall.Type != "function" {
						continue
					}
					name := toolCall.Function.Name
					if short, ok := originalToolNameMap[name]; ok {
						name = short
					} else {
						name = shortenNameIfNeeded(name)
					}
					input = append(input, map[string]any{
						"type":      "function_call",
						"call_id":   toolCall.ID,
						"name":      name,
						"arguments": toolCall.Function.Arguments,
					})
				}
			}
		}
	}
	out["input"] = input

	if textObject := buildTextObject(req.ResponseFormat, req.Text); textObject != nil {
		out["text"] = textObject
	}

	if len(req.Tools) > 0 {
		tools := make([]any, 0, len(req.Tools))
		for _, tool := range req.Tools {
			if tool.Type != "" && tool.Type != "function" {
				var passthrough any
				if err := json.Unmarshal(tool.Raw, &passthrough); err == nil {
					tools = append(tools, passthrough)
				}
				continue
			}
			if tool.Type != "function" || tool.Function == nil {
				continue
			}

			name := tool.Function.Name
			if short, ok := originalToolNameMap[name]; ok {
				name = short
			} else {
				name = shortenNameIfNeeded(name)
			}

			item := map[string]any{
				"type": "function",
				"name": name,
			}
			if tool.Function.Description != "" {
				item["description"] = tool.Function.Description
			}
			if rawJSONIsNull(tool.Function.Parameters) || len(tool.Function.Parameters) == 0 {
				item["parameters"] = defaultFunctionParameters()
			} else {
				var params any
				if err := json.Unmarshal(tool.Function.Parameters, &params); err == nil {
					item["parameters"] = params
				} else {
					item["parameters"] = defaultFunctionParameters()
				}
			}
			if tool.Function.Strict != nil {
				item["strict"] = *tool.Function.Strict
			}
			tools = append(tools, item)
		}
		out["tools"] = tools
	}

	if len(req.ToolChoice) > 0 && !rawJSONIsNull(req.ToolChoice) {
		var stringChoice string
		if err := json.Unmarshal(req.ToolChoice, &stringChoice); err == nil {
			out["tool_choice"] = stringChoice
		} else {
			var objectChoice map[string]any
			if err := json.Unmarshal(req.ToolChoice, &objectChoice); err == nil {
				toolType, _ := objectChoice["type"].(string)
				if toolType == "function" {
					name := ""
					if fn, ok := objectChoice["function"].(map[string]any); ok {
						name, _ = fn["name"].(string)
					}
					if name != "" {
						if short, ok := originalToolNameMap[name]; ok {
							name = short
						} else {
							name = shortenNameIfNeeded(name)
						}
					}
					choice := map[string]any{"type": "function"}
					if name != "" {
						choice["name"] = name
					}
					out["tool_choice"] = choice
				} else if toolType != "" {
					out["tool_choice"] = objectChoice
				}
			}
		}
	}

	output, _ := json.Marshal(out)
	return output
}

// ConvertOpenAIRequestToCodexLegacy exposes the original translator for
// equivalence tests.
func ConvertOpenAIRequestToCodexLegacy(modelName string, inputRawJSON []byte, stream bool) []byte {
	return convertOpenAIRequestToCodexLegacyImpl(modelName, inputRawJSON, stream)
}

func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 || rawJSONIsNull(raw) {
		return ""
	}
	var content string
	if err := json.Unmarshal(raw, &content); err == nil {
		return content
	}
	return string(raw)
}

func buildContentParts(role string, raw json.RawMessage) []any {
	parts := make([]any, 0)
	if len(raw) == 0 || rawJSONIsNull(raw) {
		return parts
	}

	switch firstNonSpaceByte(raw) {
	case '"':
		var content string
		if err := json.Unmarshal(raw, &content); err != nil || content == "" {
			return parts
		}
		partType := "input_text"
		if role == "assistant" {
			partType = "output_text"
		}
		return append(parts, map[string]any{
			"type": partType,
			"text": content,
		})
	case '[':
	default:
		return parts
	}

	var contentParts []chatContentPart
	if err := json.Unmarshal(raw, &contentParts); err != nil {
		return parts
	}

	for _, part := range contentParts {
		switch part.Type {
		case "text":
			partType := "input_text"
			if role == "assistant" {
				partType = "output_text"
			}
			parts = append(parts, map[string]any{
				"type": partType,
				"text": part.Text,
			})
		case "image_url":
			if role == "user" && part.ImageURL != nil && part.ImageURL.URL != "" {
				parts = append(parts, map[string]any{
					"type":      "input_image",
					"image_url": part.ImageURL.URL,
				})
			}
		case "file":
			if role == "user" && part.File != nil && part.File.FileData != "" {
				item := map[string]any{
					"type":      "input_file",
					"file_data": part.File.FileData,
				}
				if part.File.Filename != "" {
					item["filename"] = part.File.Filename
				}
				parts = append(parts, item)
			}
		}
	}

	return parts
}

func buildTextObject(responseFormat *chatRespFormat, textConfig *chatTextCfg) map[string]any {
	if responseFormat == nil && textConfig == nil {
		return nil
	}

	textObject := map[string]any{}
	if responseFormat != nil {
		format := map[string]any{}
		switch responseFormat.Type {
		case "text":
			format["type"] = "text"
		case "json_schema":
			format["type"] = "json_schema"
			if responseFormat.JSONSchema != nil {
				if responseFormat.JSONSchema.Name != "" {
					format["name"] = responseFormat.JSONSchema.Name
				}
				if responseFormat.JSONSchema.Strict != nil {
					format["strict"] = *responseFormat.JSONSchema.Strict
				}
				if len(responseFormat.JSONSchema.Schema) > 0 && !rawJSONIsNull(responseFormat.JSONSchema.Schema) {
					var schema any
					if err := json.Unmarshal(responseFormat.JSONSchema.Schema, &schema); err == nil {
						format["schema"] = schema
					}
				}
			}
		}
		if len(format) > 0 {
			textObject["format"] = format
		}
	}

	if textConfig != nil && len(textConfig.Verbosity) > 0 && !rawJSONIsNull(textConfig.Verbosity) {
		var verbosity any
		if err := json.Unmarshal(textConfig.Verbosity, &verbosity); err == nil {
			textObject["verbosity"] = verbosity
		}
	}

	if len(textObject) == 0 {
		return nil
	}
	return textObject
}

func defaultFunctionParameters() any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []any{},
	}
}

func rawJSONIsNull(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func firstNonSpaceByte(raw json.RawMessage) byte {
	for _, b := range raw {
		switch b {
		case ' ', '\n', '\r', '\t':
			continue
		default:
			return b
		}
	}
	return 0
}
