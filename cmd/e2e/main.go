// Package main 提供并行 E2E 测试运行器，从 JSON 文件读取工具输入，
// 使用信号量限流并发执行所有工具，结果保存到独立的 JSON 文件。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/config"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/logger"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit/iaas"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit/paas"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit/shared"
)

// toolResult 存储单次工具调用的输入/输出。
type toolResult struct {
	Tool      string `json:"tool"`
	Input     any    `json:"input"`
	Output    any    `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	Duration  string `json:"duration"`
	Timestamp string `json:"timestamp"`
}

func main() {
	concurrency := flag.Int("c", 5, "最大并发工具调用数")
	inputFile := flag.String("input", "tooltest/input.json", "输入 JSON 文件路径")
	outputDir := flag.String("output", "tooltest/output", "输出目录路径")
	timeout := flag.Int("timeout", 300, "单个工具超时时间（秒）")
	flag.Parse()

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.Init("info", false)

	if err := cfg.ValidateCredentials(); err != nil {
		fmt.Fprintf(os.Stderr, "Credentials error: %v\n", err)
		os.Exit(1)
	}

	cred := client.NewCredentialProvider(cfg.Credentials.AccessKeyID, cfg.Credentials.AccessKeySecret)
	slsClient := client.NewSLSClient(cred, cfg)
	cmsClient := client.NewCMSClient(cred, cfg)
	slsClient.SetCMSClient(cmsClient)

	registry := toolkit.NewRegistry()
	paasToolkit := paas.NewPaaSToolkit(cmsClient)
	iaasToolkit := iaas.NewIaaSToolkit(slsClient, cmsClient)
	sharedToolkit := shared.New(cmsClient)
	toolkit.RegisterToolkits(registry, "all", nil, paasToolkit, iaasToolkit, sharedToolkit)

	slog.Info("registered tools", "count", len(registry.List()))

	inputs, err := loadInputs(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load inputs: %v\n", err)
		os.Exit(1)
	}
	slog.Info("loaded tool inputs", "count", len(inputs))

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	perToolTimeout := time.Duration(*timeout) * time.Second
	var passed, failed, skipped atomic.Int64

	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup

	names := slices.Sorted(maps.Keys(inputs))

	for _, name := range names {
		params := inputs[name]
		tool, ok := registry.Get(name)
		if !ok {
			skipped.Add(1)
			slog.Warn("tool not registered, skipping", "tool", name)
			saveResult(filepath.Join(*outputDir, name+".json"), toolResult{
				Tool:      name,
				Input:     params,
				Error:     "skipped: tool not registered",
				Timestamp: time.Now().Format(time.RFC3339),
			})
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(name string, tool toolkit.Tool, params map[string]any) {
			defer wg.Done()
			defer func() { <-sem }()

			fmt.Printf("  START: %s\n", name)
			result := invokeTool(ctx, tool, params, perToolTimeout)
			saveResult(filepath.Join(*outputDir, name+".json"), result)

			if result.Error != "" {
				failed.Add(1)
				fmt.Printf("  FAIL:  %s — %s\n", name, result.Error)
			} else {
				passed.Add(1)
				fmt.Printf("  OK:    %s (%s)\n", name, result.Duration)
			}
		}(name, tool, params)
	}

	wg.Wait()

	p, f, s := passed.Load(), failed.Load(), skipped.Load()
	fmt.Printf("\n========== Summary ==========\n")
	fmt.Printf("Total: %d | Passed: %d | Failed: %d | Skipped: %d\n",
		p+f+s, p, f, s)
}

// loadInputs 读取并解析输入 JSON 文件。
func loadInputs(path string) (map[string]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var inputs map[string]map[string]any
	if err := json.Unmarshal(data, &inputs); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return inputs, nil
}

// invokeTool 带超时调用工具 handler 并捕获结果。
func invokeTool(ctx context.Context, tool toolkit.Tool, params map[string]any, timeout time.Duration) toolResult {
	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	output, err := tool.Handler(toolCtx, params)
	dur := time.Since(start)

	r := toolResult{
		Tool:      tool.Name,
		Input:     params,
		Duration:  dur.String(),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if err != nil {
		r.Error = err.Error()
	} else {
		r.Output = output
		// 检查工具是否在响应体中返回了错误。
		if m, ok := output.(map[string]any); ok {
			if isErr, _ := m["error"].(bool); isErr {
				if msg, _ := m["message"].(string); msg != "" {
					r.Error = msg
				}
			}
			if isErr, _ := m["isError"].(bool); isErr {
				if msg, _ := m["message"].(string); msg != "" {
					r.Error = msg
				}
			}
		}
	}

	return r
}

// saveResult 将 toolResult 写入 JSON 文件。
func saveResult(path string, r toolResult) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		slog.Error("failed to marshal result", "tool", r.Tool, "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		slog.Error("failed to write result", "path", path, "error", err)
	}
}
