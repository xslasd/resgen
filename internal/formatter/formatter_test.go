package formatter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xslasd/resgen/internal/parser"
)

func TestFormat(t *testing.T) {
	input := `# This is a module comment

module user

validator min(val: Int) # 最小值或最小长度限制
validator max(val: Int) # 最大值或最大长度限制

# A user model description

type User {
    # id comment
    id: Int # trailing comment for id
    name: String @validate(min: 2, max: 20)
}

group /users {
    # Get user by ID
    # with an empty line
    
    # in between
    GET /:id => GetUser(id: Int): User # trailing comment for endpoint
    POST /logout => Logout()
}
`

	schema, err := parser.ParseFileContent("test.res", input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	var buf bytes.Buffer
	f := NewFormatter(4)
	if err := f.Format(schema, &buf); err != nil {
		t.Fatalf("Format error: %v", err)
	}

	output := buf.String()
	t.Logf("Formatted Output:\n%s", output)

	if !strings.Contains(output, "validator min(val: Int) # 最小值或最小长度限制") {
		t.Errorf("Expected output to contain trailing comment for validator min, got:\n%s", output)
	}
	if !strings.Contains(output, "validator max(val: Int) # 最大值或最大长度限制") {
		t.Errorf("Expected output to contain trailing comment for validator max, got:\n%s", output)
	}
	if !strings.Contains(output, "    id: Int                                 # trailing comment for id") {
		t.Errorf("Expected output to contain trailing comment for id, got:\n%s", output)
	}
	if !strings.Contains(output, "GET /:id => GetUser(id: Int): User # trailing comment for endpoint") {
		t.Errorf("Expected output to contain trailing comment for endpoint, got:\n%s", output)
	}
	if !strings.Contains(output, "POST /logout => Logout()") {
		t.Errorf("Expected output to contain POST /logout => Logout(), got:\n%s", output)
	}

	// 再次解析格式化后的内容，验证是否依然合法
	_, err = parser.ParseFileContent("test_formatted.res", output)
	if err != nil {
		t.Fatalf("Parse error on formatted content: %v\nFormatted source:\n%s", err, output)
	}
}
