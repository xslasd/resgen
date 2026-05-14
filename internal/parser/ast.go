package parser

import (
	"fmt"
	"github.com/alecthomas/participle/v2/lexer"
)

// Schema 表示整个 resgen 定义文件的根节点
type Schema struct {
	Pos          lexer.Position
	Declarations []Declaration `@@*`
}

// Declaration 代表最高层级的声明
type Declaration struct {
	Pos       lexer.Position
	Module    *ModuleDecl   `  @@`
	Scalar    *ScalarDecl   `| @@`
	Decorator *MetaDecl     `| @@` // 处理 validator 和 decorator
	Model     *ModelDecl    `| @@` // 处理 type 和 input
	Group     *GroupDecl    `| @@`
}

type ModuleDecl struct {
	Pos  lexer.Position
	Doc  string
	Name string `"module" @Ident`
}

type ScalarDecl struct {
	Pos  lexer.Position
	Doc  string
	Name string `"scalar" @Ident`
}

// MetaDecl 涵盖了 validator 和 decorator
type MetaDecl struct {
	Pos     lexer.Position
	Doc     string
	IsDec   bool        `( @"decorator" | "validator" )`
	Name    string      `"@"? @Ident`
	Args    []ArgDecl   `("(" @@ ("," @@)* ")")?`
	Meta    []MetaEntry `("[" @@ ("," @@)* "]")?`
}

// ModelDecl 涵盖了 `type User { ... }` 和 `input CreateUserInput { ... }`
type ModelDecl struct {
	Pos        lexer.Position
	Doc        string
	Directives []DirectiveUsage `@@*`
	Keyword    string           `@( "input" | "type" | "wrap" )`
	Name       string           `@Ident`
	TypeParams []string         `("<" @Ident ("," @Ident)* ">")?`
	Properties []FieldDecl      `"{" @@* "}"`
}

type FieldDecl struct {
	Pos        lexer.Position
	Doc        string
	Name       string           `@Ident ":"`
	Type       TypeRef          `@@`
	Directives []DirectiveUsage `@@*`
}

// MetaEntry 表示方括号中的键值对, 如 [ctype=form, state=201, wrap=ResData]
type MetaEntry struct {
	Pos   lexer.Position
	Key   string     `@Ident "="`
	Value MetaValue  `@@`
}

// MetaValue 表示元数据的值，支持标识符、字符串、整数
type MetaValue struct {
	Pos  lexer.Position
	Str  *string `  @( Ident | String )`
	Int  *int64  `| @Int`
}

// MetaStr 返回值的字符串形式
func (v MetaValue) MetaStr() string {
	if v.Str != nil {
		return *v.Str
	}
	if v.Int != nil {
		return fmt.Sprintf("%d", *v.Int)
	}
	return ""
}

type GroupDecl struct {
	Pos        lexer.Position
	Doc        string
	Directives []DirectiveUsage `@@*`
	Name       string           `"group" ( @Ident )?`
	Path       string           `@RoutePath`
	Meta       []MetaEntry      `("[" @@ ("," @@)* "]")?`
	Endpoints  []EndpointDecl   `"{" @@* "}"`
}

type EndpointDecl struct {
	Pos          lexer.Position
	Doc          string
	Directives   []DirectiveUsage `@@*`
	Method       string           `@( "GET" | "POST" | "PUT" | "DELETE" | "PATCH" | "get" | "post" | "put" | "delete" | "patch" )`
	Path         string           `@RoutePath`
	RequestMeta  []MetaEntry      `("[" @@ ("," @@)* "]")?`
	Name         string           `"=>" @Ident`
	Args         []ArgDecl        `"(" (@@ ("," @@)*)? ")"`
	ReturnType   TypeRef          `":" @@`
	ResponseMeta []MetaEntry      `("[" @@ ("," @@)* "]")?`
}

type ArgDecl struct {
	Pos        lexer.Position
	Doc        string
	Name       string           `@Ident ":"`
	Type       TypeRef          `@@`
	Directives []DirectiveUsage `@@*`
}

type TypeRef struct {
	Pos         lexer.Position
	IsArray     bool      `@"["?`
	Name        string    `@Ident`
	TypeArgs    []TypeRef `("<" @@ ("," @@)* ">")?`
	ItemNotNull bool      `@"!"?`
	ArrEnd      bool      `@"]"?`
	ArrNotNull  bool      `@"!"?`
}

type DirectiveUsage struct {
	Pos  lexer.Position
	Name string         `"@" @Ident`
	Args []DirectiveArg `("(" @@ ("," @@)* ")")?`
	Meta []MetaEntry    `("[" @@ ("," @@)* "]")?`
}

type DirectiveArg struct {
	Pos   lexer.Position
	Name  string `( @Ident ":" )?`
	Value Value  `@@`
}

type Value struct {
	Pos    lexer.Position
	String *string  `  @String`
	Int    *int64   `| @Int`
	Float  *float64 `| @Float`
	Ident  *string  `| @Ident`
	List   []*Value `| "[" ( @@ ( "," @@ )* )? "]"`
}
