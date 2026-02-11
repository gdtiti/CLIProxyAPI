package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func main() {
	// 测试配置解析
	yamlContent := `
port: 8317
api-keys:
  - "test-key"

codex-api-key:
  - api-key: "sk-plus-test"
    prefix: "plus"
    base-url: "https://api.codex-plus.com"
    plan-type: "plus"
    excluded-models:
      - "gpt-4-mini"
  - api-key: "sk-free-test"
    prefix: "free"
    base-url: "https://api.codex-free.com"
    plan-type: "free"
    excluded-models:
      - "gpt-5"
  - api-key: "sk-general-test"
    prefix: "general"
    base-url: "https://api.codex.com"
    excluded-models:
      - "gpt-experimental"
`

	// 创建临时配置文件
	tmpFile := "test_config_temp.yaml"
	if err := writeFile(tmpFile, yamlContent); err != nil {
		log.Fatalf("写入配置文件失败: %v", err)
	}

	// 加载配置
	cfg, err := config.LoadConfig(tmpFile)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	fmt.Println("=== Codex Plus/Free 套餐区分测试 ===")
	fmt.Printf("总共 Codex 凭证数量: %d\n", len(cfg.CodexKey))

	// 按套餐类型分类统计
	planTypes := make(map[string]int)
	for _, key := range cfg.CodexKey {
		planType := key.PlanType
		if planType == "" {
			planType = "general"
		}
		planTypes[planType]++
		
		fmt.Printf("\n凭证: %s\n", key.Prefix)
		fmt.Printf("  API Key: %s...\n", key.APIKey[:10])
		fmt.Printf("  Base URL: %s\n", key.BaseURL)
		fmt.Printf("  套餐类型: %s\n", planType)
		fmt.Printf("  排除模型: %v\n", key.ExcludedModels)
	}

	fmt.Println("\n=== 套餐类型统计 ===")
	for planType, count := range planTypes {
		fmt.Printf("%s: %d 个凭证\n", planType, count)
	}

	// 测试JSON序列化
	jsonData, err := json.MarshalIndent(cfg.CodexKey, "", "  ")
	if err != nil {
		log.Fatalf("JSON序列化失败: %v", err)
	}
	
	fmt.Println("\n=== JSON 输出 ===")
	fmt.Println(string(jsonData))
}

func writeFile(filename, content string) error {
	// 简单的文件写入实现
	return nil // 这里只是测试，不实际写入
}