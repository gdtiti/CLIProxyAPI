package registry

import "testing"

func TestValidateModelsCatalogAllowsMissingOptionalProviderSections(t *testing.T) {
	catalog := &staticModelsJSON{
		Claude:    []*ModelInfo{{ID: "claude-1"}},
		Gemini:    []*ModelInfo{{ID: "gemini-1"}},
		Vertex:    []*ModelInfo{{ID: "vertex-1"}},
		GeminiCLI: []*ModelInfo{{ID: "gemini-cli-1"}},
		AIStudio:  []*ModelInfo{{ID: "aistudio-1"}},
		CodexFree: []*ModelInfo{{ID: "codex-free-1"}},
		CodexTeam: []*ModelInfo{{ID: "codex-team-1"}},
		CodexPlus: []*ModelInfo{{ID: "codex-plus-1"}},
		CodexPro:  []*ModelInfo{{ID: "codex-pro-1"}},
	}

	if err := validateModelsCatalog(catalog); err != nil {
		t.Fatalf("validateModelsCatalog() error = %v, want nil for missing optional sections", err)
	}
}

func TestValidateModelsCatalogStillRejectsMissingRequiredSection(t *testing.T) {
	catalog := &staticModelsJSON{
		Claude:    []*ModelInfo{{ID: "claude-1"}},
		Gemini:    []*ModelInfo{{ID: "gemini-1"}},
		Vertex:    []*ModelInfo{{ID: "vertex-1"}},
		GeminiCLI: []*ModelInfo{{ID: "gemini-cli-1"}},
		AIStudio:  []*ModelInfo{{ID: "aistudio-1"}},
		CodexFree: []*ModelInfo{{ID: "codex-free-1"}},
		CodexTeam: []*ModelInfo{{ID: "codex-team-1"}},
		CodexPlus: []*ModelInfo{{ID: "codex-plus-1"}},
	}

	err := validateModelsCatalog(catalog)
	if err == nil {
		t.Fatal("validateModelsCatalog() error = nil, want missing required section error")
	}
	if err.Error() != "codex-pro section is empty" {
		t.Fatalf("validateModelsCatalog() error = %v, want codex-pro section is empty", err)
	}
}
