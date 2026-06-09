package formatter

import (
	"bytes"
	"testing"

	"github.com/xslasd/resgen/internal/parser"
)

func TestFormat(t *testing.T) {
	input := `# This is a module comment

module user

# A user model description

type User {
    # id comment
    id: Int
    name: String @validate(min: 2, max: 20)
}

group /users {
    # Get user by ID
    # with an empty line
    
    # in between
    GET /:id => GetUser(id: Int): User
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

	// 再次解析格式化后的内容，验证是否依然合法
	_, err = parser.ParseFileContent("test_formatted.res", output)
	if err != nil {
		t.Fatalf("Parse error on formatted content: %v\nFormatted source:\n%s", err, output)
	}
}
