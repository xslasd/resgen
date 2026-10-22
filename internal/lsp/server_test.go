package lsp

import (
	"strings"
	"testing"

	"github.com/xslasd/resgen/internal/parser"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestDefinitionAndFormatting(t *testing.T) {
	// 模拟当前打开的文件
	uri := "file:///d:/test_workspace/user.res"
	content := `# User model
type User {
    id: Int
    name: String
}

group /users {
    GET / => GetUser(): User
}
`
	filesMu.Lock()
	files[uri] = content
	filesMu.Unlock()

	// 1. 测试从内容中查找标识符
	// "GET / => GetUser(): User" 位于第 8 行 (1-based)
	// 'U' 位于列 25 (1-based)
	filename := uriToPath(uri)
	schema, err := parser.ParseFileContent(filename, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ident := findIdentifierAt(schema, 8, 25)
	if ident != "User" {
		t.Errorf("Expected ident 'User', got '%s'", ident)
	}

	// 2. 测试建立符号表并解析
	symbols := buildSymbolTable(filename)
	sym, found := symbols["User"]
	if !found {
		t.Fatalf("Symbol 'User' not found in table")
	}
	if sym.Name != "User" || sym.Line != 2 {
		t.Errorf("Expected User at line 2, got %d", sym.Line)
	}
}

func TestAliasDirectiveDiagnostic(t *testing.T) {
	validContent := `
input FilterInput {
    startTime: String @alias("st_time")
}
`
	schema, err := parser.ParseFileContent("test.res", validContent)
	if err != nil {
		t.Fatalf("Parse validContent failed: %v", err)
	}
	if schema == nil {
		t.Fatalf("Expected non-nil schema")
	}
}

func TestCompletion(t *testing.T) {
	uri := "file:///d:/test_workspace/demo.res"
	content := "type User {\n    id: Int\n}\n\ninput QueryInput {\n    startTime: \n    name: String @"
	filesMu.Lock()
	files[uri] = content
	filesMu.Unlock()

	filename := uriToPath(uri)

	// 1. 测试在 ":" 后面的类型补全（第 5 行，startTime: 后面）
	items := buildCompletions(filename, content, 5, 15)
	var foundString, foundUser bool
	for _, item := range items {
		if item.Label == "String" {
			foundString = true
		}
		if item.Label == "User" {
			foundUser = true
		}
	}
	if !foundString || !foundUser {
		t.Errorf("Expected 'String' and 'User' in type completions, got foundString=%v, foundUser=%v", foundString, foundUser)
	}

	// 2. 测试在 "@" 后面的指令补全（第 6 行，@ 后面）
	itemsAt := buildCompletions(filename, content, 6, 18)
	var foundAlias, foundPath bool
	for _, item := range itemsAt {
		if item.Label == "@alias" {
			foundAlias = true
		}
		if item.Label == "@path" {
			foundPath = true
		}
	}
	if !foundAlias || !foundPath {
		t.Errorf("Expected '@alias' and '@path' in directive completions, got foundAlias=%v, foundPath=%v", foundAlias, foundPath)
	}

	// 3. 测试 HTTP 方法补全模版格式
	itemsRoot := buildCompletions(filename, content, 0, 0)
	var foundDelete bool
	for _, item := range itemsRoot {
		if item.Label == "DELETE" {
			foundDelete = true
			if item.InsertText == nil || *item.InsertText != "DELETE /${1:path} => ${2:func}(${3:input}): ${4:Output}" {
				t.Errorf("Unexpected DELETE insertText: %v", item.InsertText)
			}
		}
	}
	if !foundDelete {
		t.Errorf("Expected 'DELETE' in keyword completions")
	}
}

func TestHttpMethodAndDirectiveHover(t *testing.T) {
	content := `group /tasks {
    GET /list => ListTasks(): [String]
    POST /cancel => CancelTask(task_id: Int!): Boolean
    DELETE /item => DeleteItem(id: String! @query): Boolean
    PUT /update => UpdateTask(id: Int!): Boolean
    PATCH /status => PatchStatus(status: Int!): Boolean
}
`
	schema, err := parser.ParseFileContent("tasks_hover.res", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 1. GET 标识符查找 (第 2 行 "GET")
	identGet := findIdentifierAt(schema, 2, 6, content)
	if strings.ToUpper(identGet) != "GET" {
		t.Errorf("Expected ident 'GET', got '%s'", identGet)
	}

	// 2. POST 标识符查找 (第 3 行 "POST")
	identPost := findIdentifierAt(schema, 3, 6, content)
	if strings.ToUpper(identPost) != "POST" {
		t.Errorf("Expected ident 'POST', got '%s'", identPost)
	}

	// 3. DELETE 标识符查找 (第 4 行 "DELETE")
	identDelete := findIdentifierAt(schema, 4, 7, content)
	if strings.ToUpper(identDelete) != "DELETE" {
		t.Errorf("Expected ident 'DELETE', got '%s'", identDelete)
	}

	// 4. PUT 标识符查找 (第 5 行 "PUT")
	identPut := findIdentifierAt(schema, 5, 6, content)
	if strings.ToUpper(identPut) != "PUT" {
		t.Errorf("Expected ident 'PUT', got '%s'", identPut)
	}

	// 5. PATCH 标识符查找 (第 6 行 "PATCH")
	identPatch := findIdentifierAt(schema, 6, 7, content)
	if strings.ToUpper(identPatch) != "PATCH" {
		t.Errorf("Expected ident 'PATCH', got '%s'", identPatch)
	}

	// 6. @query 标识符查找 (第 4 行 "@query")
	identQuery := findIdentifierAt(schema, 4, 45, content)
	if strings.ToLower(identQuery) != "query" {
		t.Errorf("Expected ident 'query', got '%s'", identQuery)
	}

	// 7. 校验补全提示中包含自动合成 XxxArgs 包装模型的描述
	items := buildCompletions("tasks_hover.res", content, 0, 0)
	var foundPostCompletion, foundQueryDirective bool
	getDoc := func(doc any) string {
		if doc == nil {
			return ""
		}
		if s, ok := doc.(string); ok {
			return s
		}
		if mc, ok := doc.(protocol.MarkupContent); ok {
			return mc.Value
		}
		if mc, ok := doc.(*protocol.MarkupContent); ok && mc != nil {
			return mc.Value
		}
		return ""
	}
	for _, item := range items {
		docText := getDoc(item.Documentation)
		if item.Label == "POST" && strings.Contains(docText, "XxxArgs") {
			foundPostCompletion = true
		}
		if item.Label == "@query" && strings.Contains(docText, "XxxArgs") {
			foundQueryDirective = true
		}
	}
	if !foundPostCompletion {
		t.Errorf("Expected POST completion documentation to mention 'XxxArgs'")
	}
	if !foundQueryDirective {
		t.Errorf("Expected @query directive completion documentation to mention 'XxxArgs'")
	}

	// 8. 测试真实 LSP Hover 请求
	handler := NewHandler("1.0.0")
	hoverURI := "file:///d:/test_workspace/tasks_hover.res"
	filesMu.Lock()
	files[hoverURI] = content
	filesMu.Unlock()

	getHoverMarkdown := func(h *protocol.Hover) string {
		if h == nil {
			return ""
		}
		switch c := h.Contents.(type) {
		case protocol.MarkupContent:
			return c.Value
		case *protocol.MarkupContent:
			if c != nil {
				return c.Value
			}
		case string:
			return c
		}
		return ""
	}

	// 8.1 悬停在 POST 上 (Line: 2, Character: 5，即 0-based 第 2 行第 5 字符)
	hoverPost, err := handler.TextDocumentHover(nil, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: hoverURI},
			Position:     protocol.Position{Line: 2, Character: 5},
		},
	})
	if err != nil {
		t.Fatalf("TextDocumentHover failed: %v", err)
	}
	postMd := getHoverMarkdown(hoverPost)
	if !strings.Contains(postMd, "HTTP Method: `POST`") {
		t.Fatalf("Expected POST hover markdown, got: %s", postMd)
	}
	if !strings.Contains(postMd, "XxxArgs") {
		t.Errorf("Expected POST hover to explain XxxArgs wrapping")
	}

	// 8.2 悬停在 DELETE 上 (Line: 3, Character: 5)
	hoverDelete, err := handler.TextDocumentHover(nil, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: hoverURI},
			Position:     protocol.Position{Line: 3, Character: 5},
		},
	})
	if err != nil {
		t.Fatalf("TextDocumentHover failed: %v", err)
	}
	deleteMd := getHoverMarkdown(hoverDelete)
	if !strings.Contains(deleteMd, "HTTP Method: `DELETE`") {
		t.Fatalf("Expected DELETE hover markdown, got: %s", deleteMd)
	}

	// 8.3 悬停在 @query 上 (Line: 3, Character: 44)
	hoverQuery, err := handler.TextDocumentHover(nil, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: hoverURI},
			Position:     protocol.Position{Line: 3, Character: 44},
		},
	})
	if err != nil {
		t.Fatalf("TextDocumentHover failed: %v", err)
	}
	queryMd := getHoverMarkdown(hoverQuery)
	if !strings.Contains(queryMd, "Built-in Directive: `@query`") {
		t.Fatalf("Expected @query hover markdown, got: %s", queryMd)
	}
	if !strings.Contains(queryMd, "XxxArgs") {
		t.Errorf("Expected @query hover to explain non-GET XxxArgs wrapping")
	}
}
