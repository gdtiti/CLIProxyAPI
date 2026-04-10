package chat_completions

import (
	"encoding/json"
	"reflect"
	"testing"
)

func normalizeJSON(t *testing.T, data []byte) any {
	t.Helper()

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v\nraw: %s", err, data)
	}
	return value
}

func assertEquivalentToLegacy(t *testing.T, name string, input []byte, stream bool) {
	t.Helper()

	newOutput := ConvertOpenAIRequestToCodex("codex-mini-latest", input, stream)
	legacyOutput := ConvertOpenAIRequestToCodexLegacy("codex-mini-latest", input, stream)

	newValue := normalizeJSON(t, newOutput)
	legacyValue := normalizeJSON(t, legacyOutput)
	if !reflect.DeepEqual(newValue, legacyValue) {
		t.Fatalf("case %q: outputs differ\nnew:    %s\nlegacy: %s", name, newOutput, legacyOutput)
	}
}

func TestConvertOpenAIRequestToCodex_UserStringContent(t *testing.T) {
	input := []byte(`{
		"model": "codex-mini-latest",
		"messages": [
			{"role": "user", "content": "Hello, world!"}
		]
	}`)
	assertEquivalentToLegacy(t, "user-string-content", input, false)
}

func TestConvertOpenAIRequestToCodex_SystemAndUser(t *testing.T) {
	input := []byte(`{
		"messages": [
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "What is 2+2?"}
		]
	}`)
	assertEquivalentToLegacy(t, "system-and-user", input, true)
}

func TestConvertOpenAIRequestToCodex_ContentArray(t *testing.T) {
	input := []byte(`{
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "What is in this image?"},
					{"type": "image_url", "image_url": {"url": "https://example.com/img.png"}}
				]
			}
		]
	}`)
	assertEquivalentToLegacy(t, "content-array-text-image", input, false)
}

func TestConvertOpenAIRequestToCodex_ContentArrayFile(t *testing.T) {
	input := []byte(`{
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "Read this file"},
					{"type": "file", "file": {"filename": "hello.txt", "file_data": "ZmlsZQ=="}}
				]
			}
		]
	}`)
	assertEquivalentToLegacy(t, "content-array-text-file", input, false)
}

func TestConvertOpenAIRequestToCodex_AssistantToolCalls(t *testing.T) {
	input := []byte(`{
		"messages": [
			{"role": "user", "content": "What's the weather?"},
			{
				"role": "assistant",
				"content": null,
				"tool_calls": [
					{
						"id": "call_abc123",
						"type": "function",
						"function": {"name": "get_weather", "arguments": "{\"location\": \"NYC\"}"}
					}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "call_abc123",
				"content": "Sunny, 72F"
			}
		],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_weather",
					"description": "Get the current weather",
					"parameters": {
						"type": "object",
						"properties": {
							"location": {"type": "string"}
						},
						"required": ["location"]
					}
				}
			}
		]
	}`)
	assertEquivalentToLegacy(t, "assistant-tool-calls", input, false)
}

func TestConvertOpenAIRequestToCodex_ToolChoiceFunction(t *testing.T) {
	input := []byte(`{
		"messages": [
			{"role": "user", "content": "Search for me"}
		],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "search",
					"description": "Search the web",
					"parameters": {"type": "object", "properties": {}}
				}
			}
		],
		"tool_choice": {"type": "function", "function": {"name": "search"}}
	}`)
	assertEquivalentToLegacy(t, "tool-choice-function", input, false)
}

func TestConvertOpenAIRequestToCodex_ToolChoiceString(t *testing.T) {
	input := []byte(`{
		"messages": [{"role": "user", "content": "Hi"}],
		"tool_choice": "auto"
	}`)
	assertEquivalentToLegacy(t, "tool-choice-string", input, false)
}

func TestConvertOpenAIRequestToCodex_ResponseFormatJSONSchema(t *testing.T) {
	input := []byte(`{
		"messages": [{"role": "user", "content": "Give me JSON"}],
		"response_format": {
			"type": "json_schema",
			"json_schema": {
				"name": "my_schema",
				"strict": true,
				"schema": {
					"type": "object",
					"properties": {
						"result": {"type": "string"}
					},
					"required": ["result"]
				}
			}
		}
	}`)
	assertEquivalentToLegacy(t, "response-format-json-schema", input, false)
}

func TestConvertOpenAIRequestToCodex_ReasoningEffort(t *testing.T) {
	input := []byte(`{
		"messages": [{"role": "user", "content": "Think hard"}],
		"reasoning_effort": "high"
	}`)
	assertEquivalentToLegacy(t, "reasoning-effort-high", input, false)
}

func TestConvertOpenAIRequestToCodex_BuiltinTool(t *testing.T) {
	input := []byte(`{
		"messages": [{"role": "user", "content": "Search the web"}],
		"tools": [
			{"type": "web_search"}
		],
		"tool_choice": {"type": "web_search"}
	}`)
	assertEquivalentToLegacy(t, "builtin-tool", input, false)
}

func TestConvertOpenAIRequestToCodex_LongToolName(t *testing.T) {
	longName := "mcp__very_long_server_name__this_is_a_really_really_really_long_tool_name_that_exceeds_limit"
	input, err := json.Marshal(map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "use the tool"},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        longName,
					"description": "A very long tool name",
					"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal test input: %v", err)
	}

	assertEquivalentToLegacy(t, "long-tool-name", input, false)
}

func TestConvertOpenAIRequestToCodex_TextVerbosity(t *testing.T) {
	input := []byte(`{
		"messages": [{"role": "user", "content": "Be verbose"}],
		"text": {"verbosity": "verbose"}
	}`)
	assertEquivalentToLegacy(t, "text-verbosity", input, false)
}

func TestConvertOpenAIRequestToCodex_AssistantTextContent(t *testing.T) {
	input := []byte(`{
		"messages": [
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there! How can I help?"},
			{"role": "user", "content": "Tell me more"}
		]
	}`)
	assertEquivalentToLegacy(t, "assistant-text-content", input, false)
}

func TestConvertOpenAIRequestToCodex_FunctionToolWithStrict(t *testing.T) {
	input := []byte(`{
		"messages": [{"role": "user", "content": "Calculate"}],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "calculate",
					"description": "Do math",
					"strict": true,
					"parameters": {
						"type": "object",
						"properties": {
							"expression": {"type": "string"}
						},
						"required": ["expression"],
						"additionalProperties": false
					}
				}
			}
		]
	}`)
	assertEquivalentToLegacy(t, "function-tool-with-strict", input, false)
}

func TestConvertOpenAIRequestToCodex_FunctionToolDefaultsMissingParameters(t *testing.T) {
	input := []byte(`{
		"messages": [{"role": "user", "content": "Calculate"}],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "calculate",
					"description": "Do math"
				}
			}
		]
	}`)
	assertEquivalentToLegacy(t, "function-tool-default-parameters-missing", input, false)
}

func TestConvertOpenAIRequestToCodex_FunctionToolDefaultsNullParameters(t *testing.T) {
	input := []byte(`{
		"messages": [{"role": "user", "content": "Calculate"}],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "calculate",
					"description": "Do math",
					"parameters": null
				}
			}
		]
	}`)
	assertEquivalentToLegacy(t, "function-tool-default-parameters-null", input, false)
}
