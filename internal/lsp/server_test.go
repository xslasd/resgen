package lsp

import (
	"testing"

	"github.com/xslasd/resgen/internal/parser"
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
}
