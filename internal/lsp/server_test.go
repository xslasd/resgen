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
