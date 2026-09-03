package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xslasd/resgen/internal/config"
	"github.com/xslasd/resgen/internal/parser"
)

func TestGeneratorComments(t *testing.T) {
	schemaContent := `
module Blog

# 文章数据模型
type Article {
	# 文章唯一ID
	id: Int!
	title: String! # 文章标题
}

group /articles {
	# 获取单篇文章
	# id: 文章主键ID
	GET /:id => GetArticle(id: Int @path): Article
}
`
	schema, err := parser.ParseFileContent("test.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-gen-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package: "testpkg",
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 检查生成的 blog.gen.go
	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "blog.gen.go"))
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	code := string(contentBytes)

	// 验证结构体注释
	if !strings.Contains(code, "// 文章数据模型") {
		t.Errorf("generated code does not contain model comment '// 文章数据模型'")
	}
	// 验证字段注释
	if !strings.Contains(code, "// 文章唯一ID") {
		t.Errorf("generated code does not contain field comment '// 文章唯一ID'")
	}
	if !strings.Contains(code, "// 文章标题") {
		t.Errorf("generated code does not contain field comment '// 文章标题'")
	}
	// 验证接口注释
	if !strings.Contains(code, "// 获取单篇文章") {
		t.Errorf("generated code does not contain endpoint comment '// 获取单篇文章'")
	}
	// 验证参数说明
	if !strings.Contains(code, "@param id 文章主键ID") {
		t.Errorf("generated code does not contain param doc '@param id 文章主键ID'")
	}
	// 验证路由说明
	if !strings.Contains(code, "// GET /articles/:id") {
		t.Errorf("generated code does not contain route path '// GET /articles/:id'")
	}
}

func TestCustomInitialisms_PreserveCase(t *testing.T) {
	schemaContent := `
module System

input BatchDeleteInput {
	gids: [String!]!
	menu_gid: String!
}

group /system {
	POST /batch-delete => BatchDelete(input: BatchDeleteInput): Void
}
`
	schema, err := parser.ParseFileContent("system.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-initialism-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package:       "system",
			GoInitialisms: []string{"GIDs"},
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "system.gen.go"))
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	code := string(contentBytes)

	// 验证 gids 保留用户配置的 GIDs，而不是变成全大写的 GIDS
	if !strings.Contains(code, "GIDs") || strings.Contains(code, "GIDS") {
		t.Errorf("expected 'GIDs', but generated code was:\n%s", code)
	}
	// 验证 menu_gid 依然正常保持内置的 MenuGID
	if !strings.Contains(code, "MenuGID") {
		t.Errorf("expected 'MenuGID', but generated code was:\n%s", code)
	}
}

func TestGlobalEnum_InEngineGen(t *testing.T) {
	commonSchema := `
# 通用布尔检索三态枚举
enum BooleanSearch: Int {
  All: -1
  True: 1
  False: 0
}
`
	bizSchema := `
module UserBiz

input QueryUserInput {
  tf_disable: BooleanSearch
}

group /user {
  POST /query => QueryUser(input: QueryUserInput): Void
}
`
	s1, err := parser.ParseFileContent("00_common.res", commonSchema)
	if err != nil {
		t.Fatalf("Parse 00_common.res failed: %v", err)
	}
	s2, err := parser.ParseFileContent("01_biz.res", bizSchema)
	if err != nil {
		t.Fatalf("Parse 01_biz.res failed: %v", err)
	}

	mergedSchema := &parser.Schema{
		Declarations: append(s1.Declarations, s2.Declarations...),
	}

	tmpDir, err := os.MkdirTemp("", "resgen-global-enum-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package:       "resolver",
			EnableApiDocs: true,
		},
	}

	if err := Generate(mergedSchema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 1. 验证 engine.gen.go 包含 BooleanSearch 全局枚举声明及相关方法
	engineBytes, err := os.ReadFile(filepath.Join(tmpDir, "engine.gen.go"))
	if err != nil {
		t.Fatalf("failed to read engine.gen.go: %v", err)
	}
	engineCode := string(engineBytes)

	if !strings.Contains(engineCode, "type BooleanSearch int64") {
		t.Errorf("engine.gen.go missing 'type BooleanSearch int64'")
	}
	if !strings.Contains(engineCode, "BooleanSearch_All") {
		t.Errorf("engine.gen.go missing BooleanSearch_All, engineCode was:\n%s", engineCode)
	}
	if !strings.Contains(engineCode, "func (e BooleanSearch) IsValid() bool") {
		t.Errorf("engine.gen.go missing IsValid() method on BooleanSearch")
	}
	if !strings.Contains(engineCode, "func (e *BooleanSearch) FromParam(ctx any, s string) error") {
		t.Errorf("engine.gen.go missing FromParam() method on BooleanSearch")
	}

	// 2. 验证 userbiz.gen.go 引用了 BooleanSearch
	bizBytes, err := os.ReadFile(filepath.Join(tmpDir, "userbiz.gen.go"))
	if err != nil {
		t.Fatalf("failed to read userbiz.gen.go: %v", err)
	}
	bizCode := string(bizBytes)

	if !strings.Contains(bizCode, "TfDisable *BooleanSearch") {
		t.Errorf("userbiz.gen.go missing 'TfDisable *BooleanSearch'")
	}

	// 3. 验证 docs/api.json 符合 OpenAPI 3.0.3 规范且包含 x-res-file
	apiJsonBytes, err := os.ReadFile(filepath.Join(tmpDir, "docs", "api.json"))
	if err != nil {
		t.Fatalf("failed to read api.json: %v", err)
	}
	apiJson := string(apiJsonBytes)

	if !strings.Contains(apiJson, `"openapi": "3.0.3"`) {
		t.Errorf("api.json missing '\"openapi\": \"3.0.3\"'")
	}
	if !strings.Contains(apiJson, `"paths": {`) {
		t.Errorf("api.json missing '\"paths\": {'")
	}
	if !strings.Contains(apiJson, `"x-res-file": "01_biz.res"`) {
		t.Errorf("api.json missing '\"x-res-file\": \"01_biz.res\"'")
	}
	if !strings.Contains(apiJson, `"x-res-file": "00_common.res"`) {
		t.Errorf("api.json missing '\"x-res-file\": \"00_common.res\"'")
	}

	// 4. 验证 docs/api.html 正确生成
	apiHtmlBytes, err := os.ReadFile(filepath.Join(tmpDir, "docs", "api.html"))
	if err != nil {
		t.Fatalf("failed to read api.html: %v", err)
	}
	if len(apiHtmlBytes) == 0 {
		t.Errorf("api.html is empty")
	}
}


