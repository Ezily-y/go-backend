package docscheck

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"go-backend/docs"
	"go-backend/internal/bootstrap"
	"go-backend/internal/core/config"
	"go-backend/internal/core/database"
)

// skipRoutes 是不需要写进 API 文档的基础设施端点：
// 运维探查、静态文件、文档自身 —— 对外提供业务契约的接口才需要 spec。
var skipRoutes = map[string]bool{
	"/health":            true, // 运维探针
	"/ready":             true, // 运维探针
	"/metrics":           true, // Prometheus 抓取
	"/docs":              true, // 文档 UI 自身
	"/openapi.json":      true, // 文档 spec 自身
	"/uploads/*filepath": true, // 静态文件
}

// TestSpecCoversAllRoutes 是防漂移门禁。
//
// 背景：OpenAPI spec 由 docs 包手写（Go struct），与 router 是两份独立的事实源，
// 新增接口时容易只改 router 而漏掉文档 —— 编译和原有测试都不会发现。
// 本测试双向比对，把"忘了写文档"变成 go test 直接失败。
//
// 新增 API 的完整流程见 .claude/skills/go-backend-api-doc/SKILL.md。
func TestSpecCoversAllRoutes(t *testing.T) {
	engineRoutes := engineRouteSet(t)
	specOps := specOperationSet()

	var missing, stale []string

	// 方向一：router 有、spec 没有 —— 文档漏写
	for key := range engineRoutes {
		if _, ok := specOps[key]; !ok {
			missing = append(missing, key)
		}
	}
	// 方向二：spec 有、router 没有 —— 文档写了已删除/写错的接口
	for key := range specOps {
		if _, ok := engineRoutes[key]; !ok {
			stale = append(stale, key)
		}
	}

	sort.Strings(missing)
	sort.Strings(stale)

	if len(missing) > 0 {
		t.Errorf("以下路由已注册但 OpenAPI spec 中缺失（新增接口漏写文档）:\n  %s\n"+
			"修复: 在 docs/paths.go 补 p.Set(...)，必要时在 docs/schemas.go 补 schema。\n"+
			"流程见 .claude/skills/go-backend-api-doc/SKILL.md",
			strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("以下 spec 条目在路由中不存在（接口已删除或路径写错）:\n  %s\n"+
			"修复: 从 docs/paths.go 删除，或修正路径与 HTTP 方法。",
			strings.Join(stale, "\n  "))
	}

	t.Logf("engine routes=%d (去基础设施 %d), spec operations=%d",
		len(engineRoutes)+len(skipRoutes), len(engineRoutes), len(specOps))
}

// TestSpecSchemasWellFormed 验证 spec 能被独立解析器接受 ——
// 抓的是 $ref 悬空、schema 结构非法这类"构建时能过、外部工具打不开"的问题。
func TestSpecSchemasWellFormed(t *testing.T) {
	raw := docs.JSON()

	// 用 kin-openapi 自己的 Loader 走一遍 JSON roundtrip，
	// 等价于第三方工具（Apifox / Scalar / redoc）加载这份 spec。
	loader := &openapi3.Loader{IsExternalRefsAllowed: false}
	doc, err := loader.LoadFromData(raw)
	if err != nil {
		t.Fatalf("spec 无法被解析（$ref 悬空或结构非法）: %v", err)
	}
	if doc.OpenAPI != "3.1.0" {
		t.Errorf("OpenAPI 版本 = %q, want 3.1.0", doc.OpenAPI)
	}
	if doc.Paths.Len() == 0 {
		t.Error("spec 中没有任何 paths")
	}
}

// engineRouteSet 起真实 engine，返回需要写文档的 "METHOD path" 集合。
func engineRouteSet(t *testing.T) map[string]bool {
	t.Helper()

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	// 用临时 sqlite，避免测试污染 ./data/app.db
	cfg.Database.DSN = filepath.Join(t.TempDir(), "drift.db")
	cfg.App.Mode = "test"

	app, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("装配应用失败: %v", err)
	}
	// 关掉数据库句柄，否则 Windows 下 TempDir 清理会因文件被占用而失败。
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Logf("关闭测试数据库失败: %v", err)
		}
	})

	out := make(map[string]bool)
	for _, r := range app.Engine.Routes() {
		if skipRoutes[r.Path] {
			continue
		}
		out[r.Method+" "+r.Path] = true
	}
	return out
}

// specOperationSet 返回 spec 里 "METHOD /path" 集合（路径已转成 gin 风格）。
//
// 同样跳过 skipRoutes —— /health、/ready 这类基础设施端点 spec 里允许写
// （对运维有用），但不参与漂移比对，否则会误报为"路由中不存在"。
func specOperationSet() map[string]bool {
	out := make(map[string]bool)
	spec := docs.Spec()
	for _, path := range spec.Paths.Keys() {
		if skipRoutes[path] {
			continue
		}
		item := spec.Paths.Find(path)
		for method, op := range map[string]*openapi3.Operation{
			"GET": item.Get, "POST": item.Post, "PUT": item.Put,
			"PATCH": item.Patch, "DELETE": item.Delete, "HEAD": item.Head,
			"OPTIONS": item.Options,
		} {
			if op == nil {
				continue
			}
			// spec 路径 {id} -> gin 的 :id，与 engine 侧对齐
			ginPath := regexp.MustCompile(`\{(\w+)\}`).ReplaceAllString(path, ":$1")
			out[method+" "+ginPath] = true
		}
	}
	return out
}
