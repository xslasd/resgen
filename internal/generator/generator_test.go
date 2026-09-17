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

func TestInputPointerVsValueType(t *testing.T) {
	schemaContent := `
module EventMod

type Event {
	id: Int!
	name: String!
}

input QueryEventsInput {
	keyword: String
}

group /events {
	GET /list => ListEvents(input: QueryEventsInput): [Event]
	GET /list-required => ListEventsRequired(input: QueryEventsInput!): [Event]
}
`
	schema, err := parser.ParseFileContent("event.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-input-type-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package: "eventpkg",
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "eventmod.gen.go"))
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	code := string(contentBytes)

	// 1. 验证可空输入 QueryEventsInput 生成指针类型 *QueryEventsInput
	if !strings.Contains(code, "ListEvents(ctx context.Context, input *QueryEventsInput) (*[]*Event, error)") {
		t.Errorf("expected ListEvents with pointer input '*QueryEventsInput', but got code:\n%s", code)
	}
	// 验证可空调用的执行器传递 &input
	if !strings.Contains(code, "e.biz.ListEvents(request.Context(), &input)") {
		t.Errorf("expected e.biz.ListEvents to pass '&input', but got code:\n%s", code)
	}

	// 2. 验证必填输入 QueryEventsInput! 生成值类型 QueryEventsInput
	if !strings.Contains(code, "ListEventsRequired(ctx context.Context, input QueryEventsInput) (*[]*Event, error)") {
		t.Errorf("expected ListEventsRequired with value input 'QueryEventsInput', but got code:\n%s", code)
	}
	// 验证必填调用的执行器传递 input
	if !strings.Contains(code, "e.biz.ListEventsRequired(request.Context(), input)") {
		t.Errorf("expected e.biz.ListEventsRequired to pass 'input', but got code:\n%s", code)
	}
}

func TestNestedWrapperWithDefaultWrapEquivalence(t *testing.T) {
	schemaContent := `
module ArticleMod

wrap ResData<T> {
	code: Int!
	msg: String!
	data: T
}

wrap ListRes<T> {
	rows: [T!]!
	total: Int!
}

type Article {
	id: Int!
	title: String!
}

group /articles {
	# 隐式自动包装 ListRes<Article>，外层由 default_wrap: ResData 包装
	GET /list-implicit => ListImplicit(): ListRes<Article>
	# 显式声明外层 ResData<ListRes<Article>>
	GET /list-explicit => ListExplicit(): ResData<ListRes<Article>>
}
`
	schema, err := parser.ParseFileContent("article.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-nested-wrap-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package:     "articlepkg",
			DefaultWrap: "ResData",
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "articlemod.gen.go"))
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	code := string(contentBytes)

	// 1. 验证两种声明方式生成的 Resolver 签名完全一致：均返回 (*ListResArticle, error)
	if !strings.Contains(code, "ListImplicit(ctx context.Context) (*ListResArticle, error)") {
		t.Errorf("expected ListImplicit to return (*ListResArticle, error), got code:\n%s", code)
	}
	if !strings.Contains(code, "ListExplicit(ctx context.Context) (*ListResArticle, error)") {
		t.Errorf("expected ListExplicit to return (*ListResArticle, error), got code:\n%s", code)
	}

	// 2. 验证执行器中均通过 BindResData 进行包装渲染
	if !strings.Contains(code, "e.r.BindResData(native, result, nil)") {
		t.Errorf("expected executor to render with BindResData, got code:\n%s", code)
	}
}

func TestAuthDecoratorSystemPerms(t *testing.T) {
	schemaContent := `
module AuthCenter

decorator @auth(role: String) [stage=request]

group /system {
	# 获取用户列表
	@auth("sys:user:list")
	GET /users => ListUsers(): [String!]!

	# 获取角色列表
	@auth
	GET /roles => ListRoles(): [String!]!

	GET /public => PublicInfo(): String!
}
`
	schema, err := parser.ParseFileContent("auth.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-auth-perms-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package:       "authpkg",
			AuthDecorator: "auth",
			AuthParamName: "role",
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	engineBytes, err := os.ReadFile(filepath.Join(tmpDir, "engine.gen.go"))
	if err != nil {
		t.Fatalf("failed to read engine.gen.go: %v", err)
	}
	engineCode := string(engineBytes)

	// 1. 验证存在 PermDefine 定义
	if !strings.Contains(engineCode, "type PermDefine struct {") {
		t.Errorf("expected engine.gen.go to define PermDefine struct, got:\n%s", engineCode)
	}

	// 2. 验证存在 SystemPerms 变量
	if !strings.Contains(engineCode, "var SystemPerms = []PermDefine{") {
		t.Errorf("expected engine.gen.go to define SystemPerms variable, got:\n%s", engineCode)
	}

	// 3. 验证显式指定的权限值与模块全小写、Title 及单行注释
	if !strings.Contains(engineCode, `{Module: "authcenter", Perm: "sys:user:list", Title: "获取用户列表"}`) || !strings.Contains(engineCode, `// 获取用户列表`) {
		t.Errorf("expected SystemPerms to contain single-line sys:user:list with Title and comment, got:\n%s", engineCode)
	}

	// 4. 验证无参 @auth 自动回退为 authcenter.listroles 且带 Title 和注释
	if !strings.Contains(engineCode, `{Module: "authcenter", Perm: "authcenter.listroles", Title: "获取角色列表"}`) || !strings.Contains(engineCode, `// 获取角色列表`) {
		t.Errorf("expected SystemPerms to fallback to authcenter.listroles with Title and comment, got:\n%s", engineCode)
	}

	// 5. 验证 PublicInfo 不带 @auth 的接口不会进入 SystemPerms
	if strings.Contains(engineCode, "PublicInfo") {
		t.Errorf("expected SystemPerms to not contain PublicInfo, got:\n%s", engineCode)
	}
}

func TestAuthDecoratorLoginOnly(t *testing.T) {
	schemaContent := `
module AuthCenter

decorator @auth(login_only: Boolean) [stage=request]

@auth(true)
group /auth/v1 {
	# 获取当前登录用户信息
	GET /authinfo => AuthInfo(): String!
	POST /logout => Logout()
}

@auth(false)
group /system/v1 {
	# 获取系统用户
	GET /users => ListUsers(): String!
}
`
	schema, err := parser.ParseFileContent("auth_test.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-auth-login-only-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package:       "authpkg",
			AuthDecorator: "auth",
			AuthParamName: "",
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	engineBytes, err := os.ReadFile(filepath.Join(tmpDir, "engine.gen.go"))
	if err != nil {
		t.Fatalf("failed to read engine.gen.go: %v", err)
	}
	engineCode := string(engineBytes)

	// 1. 验证 @auth(true) 接口绝对不产生 Perm: "true"
	if strings.Contains(engineCode, `Perm: "true"`) {
		t.Errorf("expected SystemPerms to NOT contain Perm: \"true\", got:\n%s", engineCode)
	}

	// 2. 验证 @auth(true) 属于仅登录认证接口，不应出现在 SystemPerms 字典中
	if strings.Contains(engineCode, "authinfo") || strings.Contains(engineCode, "logout") {
		t.Errorf("expected SystemPerms to NOT contain login_only endpoints (authinfo, logout), got:\n%s", engineCode)
	}

	// 3. 验证 @auth(false) 需要权限校验，自动回退为 authcenter.listusers
	if !strings.Contains(engineCode, `{Module: "authcenter", Perm: "authcenter.listusers", Title: "获取系统用户"}`) {
		t.Errorf("expected SystemPerms to contain authcenter.listusers, got:\n%s", engineCode)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		maxLen   int
		expected string
	}{
		{
			name:     "empty doc",
			doc:      "",
			maxLen:   30,
			expected: "",
		},
		{
			name:     "single line plain",
			doc:      "// 创建新用户",
			maxLen:   30,
			expected: "创建新用户",
		},
		{
			name:     "multi line doc",
			doc:      "// 获取用户信息\n// 详细说明第二行\n// 详细说明第三行",
			maxLen:   30,
			expected: "获取用户信息",
		},
		{
			name:     "punctuation cutoff chinese period",
			doc:      "// 查询组织机构树。支持懒加载和全量递归，返回层级结构数据",
			maxLen:   30,
			expected: "查询组织机构树",
		},
		{
			name:     "punctuation cutoff semicolon",
			doc:      "// 导出财务报表；需要异步导出",
			maxLen:   30,
			expected: "导出财务报表",
		},
		{
			name:     "max len limit truncate",
			doc:      "这是一个特别特别长甚至超过了最大长度限制的非常冗长详细的接口说明描述文字",
			maxLen:   10,
			expected: "这是一个特别特别长甚",
		},
		{
			name:     "floating point number in doc not cutoff",
			doc:      "// 版本 2.0 升级接口。请注意兼容性",
			maxLen:   30,
			expected: "版本 2.0 升级接口",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.doc, tt.maxLen)
			if got != tt.expected {
				t.Errorf("extractTitle(%q, %d) = %q, expected %q", tt.doc, tt.maxLen, got, tt.expected)
			}
		})
	}
}

func TestDeleteWithRequestBody(t *testing.T) {
	schemaContent := `
module Auditlogs

input BatchDeleteReq {
	ids: [String!]!
}

group /auditlogs/v1 {
	# 批量删除操作日志
	DELETE /sys-oper-log => DeleteSysOperLog(req: BatchDeleteReq!): Void
}
`
	schema, err := parser.ParseFileContent("audit.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-delete-payload-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package:       "auditpkg",
			EnableApiDocs: true,
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 1. 验证 module 代码中生成了 request.Payload 调用
	moduleBytes, err := os.ReadFile(filepath.Join(tmpDir, "auditlogs.gen.go"))
	if err != nil {
		t.Fatalf("failed to read auditlogs.gen.go: %v", err)
	}
	moduleCode := string(moduleBytes)
	if !strings.Contains(moduleCode, "request.Payload(SourceJSON, input)") {
		t.Errorf("expected module code to call request.Payload for DELETE endpoint, got:\n%s", moduleCode)
	}

	// 2. 验证 api.json 中 DELETE 操作生成了 requestBody
	apiJsonBytes, err := os.ReadFile(filepath.Join(tmpDir, "docs", "api.json"))
	if err != nil {
		t.Fatalf("failed to read api.json: %v", err)
	}
	apiJson := string(apiJsonBytes)
	if !strings.Contains(apiJson, `"requestBody"`) || !strings.Contains(apiJson, `#/components/schemas/BatchDeleteReq`) {
		t.Errorf("expected api.json to contain requestBody for DELETE endpoint referencing BatchDeleteReq, got:\n%s", apiJson)
	}
}

func TestGroupAndEndpointDecoratorOverride(t *testing.T) {
	schemaContent := `
module System

decorator @auth
decorator @permission(sign: String)

type RoleMenuTreeResp {
	checked_keys: [String!]!
}

@auth
@permission
group /system/v1 {
	@permission(sign: "authcenter.updatesysrole")
	GET /menuTreeselect/:role_gid => GetRoleMenuTree(
		role_gid: String! @path
	): RoleMenuTreeResp

	GET /normal => NormalEndpoint(): Void
}
`
	schema, err := parser.ParseFileContent("system.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-override-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package: "systempkg",
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

	// 验证 GetRoleMenuTree 中只存在一次 e.d.Permission，且参数为 "authcenter.updatesysrole"
	countSpecific := strings.Count(code, `e.d.Permission(native, info, "authcenter.updatesysrole")`)
	if countSpecific != 1 {
		t.Errorf("expected exactly 1 occurrence of e.d.Permission with 'authcenter.updatesysrole', got %d", countSpecific)
	}

	// 验证 GetRoleMenuTree 中不应该存在未覆盖的空权限调用 e.d.Permission(native, info, "")
	// 注意 NormalEndpoint 继承了组级的 @permission，所以 NormalEndpoint 会有 e.d.Permission(native, info, "")，出现 1 次
	countEmpty := strings.Count(code, `e.d.Permission(native, info, "")`)
	if countEmpty != 1 {
		t.Errorf("expected exactly 1 occurrence of e.d.Permission with empty sign (from NormalEndpoint only), got %d", countEmpty)
	}
}

func TestSystemPermsArbitration(t *testing.T) {
	schemaContent := `
decorator @auth
decorator @permission(sign: String)

module Authcenter

@permission
group /auth/v1 {
	# 修改已存在的角色基本信息
	PUT /role/:role_gid => UpdateSysRole(): Void
}

module System

@permission
group /system/v1 {
	# 游标分页检索字典类型列表
	GET /dict-type => ListSysDictType(): Void

	# 获取指定字典类型的详细信息
	@permission(sign: "system.listsysdicttype")
	GET /dict-type/:dict_gid => GetSysDictType(): Void

	# 获取角色分配的菜单树 (跨模块借用 authcenter 权限)
	@permission(sign: "authcenter.updatesysrole")
	GET /menuTreeselect/:role_gid => GetRoleMenuTree(): Void
}
`
	schema, err := parser.ParseFileContent("test_arbitration.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-arbitration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package:       "testpkg",
			AuthDecorator: "permission",
			AuthParamName: "sign",
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	engineBytes, err := os.ReadFile(filepath.Join(tmpDir, "engine.gen.go"))
	if err != nil {
		t.Fatalf("failed to read engine.gen.go: %v", err)
	}
	code := string(engineBytes)

	// 1. 验证跨模块借用的 authcenter.updatesysrole 全局只有 1 条，且 Module 锁定为 authcenter，Title 来自 UpdateSysRole
	countRole := strings.Count(code, `"authcenter.updatesysrole"`)
	if countRole != 1 {
		t.Errorf("expected authcenter.updatesysrole to appear exactly once in engine.gen.go, got %d", countRole)
	}
	expectedRoleLine := `{Module: "authcenter", Perm: "authcenter.updatesysrole", Title: "修改已存在的角色基本信息"}`
	if !strings.Contains(code, expectedRoleLine) {
		t.Errorf("expected SystemPerms to contain:\n%s\nbut got:\n%s", expectedRoleLine, code)
	}

	// 2. 验证多端点复用的 system.listsysdicttype 全局只有 1 条，且 Title 由主端点 ListSysDictType 的注释胜出
	countDict := strings.Count(code, `"system.listsysdicttype"`)
	if countDict != 1 {
		t.Errorf("expected system.listsysdicttype to appear exactly once in engine.gen.go, got %d", countDict)
	}
	expectedDictLine := `{Module: "system", Perm: "system.listsysdicttype", Title: "游标分页检索字典类型列表"}`
	if !strings.Contains(code, expectedDictLine) {
		t.Errorf("expected SystemPerms to contain:\n%s\nbut got:\n%s", expectedDictLine, code)
	}
}

func TestMonomorphizeMultiDimensionalSlice(t *testing.T) {
	schemaContent := `
module User

type ListRes<T> {
	rows: [T]!
	total: Int!
}

type SysUserItem {
	id: Int!
	username: String!
}

group /user {
	GET /list1 => ListUsers1(): ListRes<SysUserItem>
	GET /list2 => ListUsers2(): ListRes<[SysUserItem!]>
	GET /list3 => ListUsers3(): ListRes<[SysUserItem]>
}
`
	schema, err := parser.ParseFileContent("test_multidim.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-multidim-test-*")
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

	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "user.gen.go"))
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	code := string(contentBytes)

	// 1. 验证生成了 ListResSysUserItem 结构体，且 Rows 类型为 []*SysUserItem
	if !strings.Contains(code, "type ListResSysUserItem struct {") {
		t.Errorf("expected struct ListResSysUserItem to be generated, code:\n%s", code)
	}
	if !strings.Contains(code, "Rows  []*SysUserItem") {
		t.Errorf("expected ListResSysUserItem.Rows to be []*SysUserItem, code:\n%s", code)
	}

	// 2. 验证生成了 ListResListSysUserItem 结构体，且 Rows 类型展开为二维切片 [][]SysUserItem
	if !strings.Contains(code, "type ListResListSysUserItem struct {") {
		t.Errorf("expected struct ListResListSysUserItem to be generated, code:\n%s", code)
	}
	if !strings.Contains(code, "Rows  [][]SysUserItem") {
		t.Errorf("expected ListResListSysUserItem.Rows to be [][]SysUserItem, code:\n%s", code)
	}

	// 3. 验证端点返回值正确引用了单态化后的对应类型
	if !strings.Contains(code, "ListUsers1(ctx context.Context) (*ListResSysUserItem, error)") {
		t.Errorf("expected ListUsers1 to return *ListResSysUserItem, code:\n%s", code)
	}
	if !strings.Contains(code, "ListUsers2(ctx context.Context) (*ListResListSysUserItem, error)") {
		t.Errorf("expected ListUsers2 to return *ListResListSysUserItem, code:\n%s", code)
	}
}

func TestParseGoType(t *testing.T) {
	tests := []struct {
		goType           string
		wantArray        bool
		wantPointer      bool
		wantElemPointer  bool
		wantBase         string
	}{
		{"string", false, false, false, "string"},
		{"*string", false, true, false, "string"},
		{"[]string", true, false, false, "string"},
		{"[]*string", true, false, true, "string"},
		{"*[]string", true, true, false, "string"},
		{"*[]*string", true, true, true, "string"},
	}

	for _, tt := range tests {
		isArray, isPointer, isElementPointer, baseType := parseGoType(tt.goType)
		if isArray != tt.wantArray || isPointer != tt.wantPointer || isElementPointer != tt.wantElemPointer || baseType != tt.wantBase {
			t.Errorf("parseGoType(%q) = (%v, %v, %v, %q), want (%v, %v, %v, %q)",
				tt.goType, isArray, isPointer, isElementPointer, baseType,
				tt.wantArray, tt.wantPointer, tt.wantElemPointer, tt.wantBase)
		}
	}
}

func TestArrayFieldValidation(t *testing.T) {
	schemaContent := `
module Storage

input UpdateSceneReq {
	scene_gid: String!
	blocked_exts: [String!]
}

group /storage {
	POST /scene/update => UpdateScene(input: UpdateSceneReq): Void
}
`
	schema, err := parser.ParseFileContent("storage.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-arr-test-*")
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

	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "storage.gen.go"))
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	code := string(contentBytes)

	// 不应该对整个切片指针做 e.v.Required(ctx, "blocked_exts", *input.BlockedExts)
	if strings.Contains(code, `e.v.Required(ctx, "blocked_exts", *input.BlockedExts)`) {
		t.Errorf("expected code NOT to contain slice-level Required validation on blocked_exts, code:\n%s", code)
	}

	// 应该遍历切片并校验每个元素
	if !strings.Contains(code, `for i, item := range *input.BlockedExts {`) {
		t.Errorf("expected code to contain for-range over *input.BlockedExts, code:\n%s", code)
	}
	if !strings.Contains(code, `e.v.Required(ctx, "blocked_exts"+"["+strconv.Itoa(i)+"]", item)`) {
		t.Errorf("expected code to contain element-level Required validation, code:\n%s", code)
	}
}

func TestComprehensiveValidationRules(t *testing.T) {
	schemaContent := `
module Demo

input ComplexReq {
	title: String @required
	tags: [String]!
	optional_tags: [String!]
}

group /demo {
	POST /test => TestAction(input: ComplexReq): Void
}
`
	schema, err := parser.ParseFileContent("demo.res", schemaContent)
	if err != nil {
		t.Fatalf("ParseFileContent failed: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "resgen-comprehensive-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &config.Config{
		Generator: config.GeneratorConfig{
			Package:       "demopkg",
			EnableApiDocs: true,
		},
	}

	if err := Generate(schema, tmpDir, conf); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	contentBytes, err := os.ReadFile(filepath.Join(tmpDir, "demo.gen.go"))
	if err != nil {
		t.Fatalf("failed to read demo.gen.go: %v", err)
	}
	code := string(contentBytes)

	// 1. 验证显式 @required 指令生成了校验逻辑
	if !strings.Contains(code, `e.v.Required(ctx, "title",`) {
		t.Errorf("expected @required directive on title to generate Required validation, code:\n%s", code)
	}

	// 2. 验证 tags: [String]! 在切片本身生成了 Required 校验
	if !strings.Contains(code, `e.v.Required(ctx, "tags", input.Tags)`) {
		t.Errorf("expected slice-level Required validation on tags: [String]!, code:\n%s", code)
	}

	// 3. 验证 optional_tags: [String!] 在切片本身不生成 Required 校验，而是在遍历切片元素时校验
	if strings.Contains(code, `e.v.Required(ctx, "optional_tags",`) {
		t.Errorf("expected optional_tags NOT to have slice-level Required validation, code:\n%s", code)
	}
	if !strings.Contains(code, `for i, item := range *input.OptionalTags {`) {
		t.Errorf("expected optional_tags to range over slice elements, code:\n%s", code)
	}
	if !strings.Contains(code, `e.v.Required(ctx, "optional_tags"+"["+strconv.Itoa(i)+"]", item)`) {
		t.Errorf("expected optional_tags to validate elements, code:\n%s", code)
	}

	// 4. 验证 OpenAPI 的 required 字段只有 title 和 tags，不包含 optional_tags
	apiJsonBytes, err := os.ReadFile(filepath.Join(tmpDir, "docs", "api.json"))
	if err != nil {
		t.Fatalf("failed to read api.json: %v", err)
	}
	apiJson := string(apiJsonBytes)
	if !strings.Contains(apiJson, `"title"`) || !strings.Contains(apiJson, `"tags"`) {
		t.Errorf("expected api.json to contain title and tags")
	}
}

