package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// ParseSchema 智能解析：支持多个文件或目录，合并所有声明后进行校验
func ParseSchema(paths ...string) (*Schema, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if info.IsDir() {
			err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && filepath.Ext(p) == ".res" {
					files = append(files, p)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			files = append(files, path)
		}
	}

	globalSchema := &Schema{Declarations: make([]Declaration, 0)}

	for _, file := range files {
		fmt.Printf("Parsing schema file: %s\n", file)
		
		// 1. 预处理：提取注释
		comments, err := collectComments(file)
		if err != nil {
			return nil, err
		}

		// 2. 解析 AST
		schema, err := ParseFile(file)
		if err != nil {
			return nil, err
		}

		// 3. 将注释挂载到 AST 节点
		attachCommentsToSchema(schema, comments)

		globalSchema.Declarations = append(globalSchema.Declarations, schema.Declarations...)
	}

	if err := globalSchema.Validate(); err != nil {
		return nil, err
	}

	return globalSchema, nil
}

func collectComments(filename string) (map[int]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	comments := make(map[int]string)
	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			comments[lineNum] = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
		lineNum++
	}
	return comments, scanner.Err()
}

func attachCommentsToSchema(s *Schema, comments map[int]string) {
	for i := range s.Declarations {
		decl := &s.Declarations[i]
		line := decl.Pos.Line
		// 寻找节点上方的注释块
		var sb strings.Builder
		for l := line - 1; l > 0; l-- {
			if txt, ok := comments[l]; ok {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(txt)
			} else {
				// 如果某一行不是注释且不是空行，停止向上搜寻
				break
			}
		}
		// 翻转顺序（因为是向上搜寻的）
		parts := strings.Split(sb.String(), "\n")
		var finalDoc []string
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" {
				finalDoc = append(finalDoc, parts[i])
			}
		}
		doc := strings.Join(finalDoc, "\n")

		if decl.Module != nil { decl.Module.Doc = doc }
		if decl.Scalar != nil { decl.Scalar.Doc = doc }
		if decl.Decorator != nil { decl.Decorator.Doc = doc }
		if decl.Model != nil { 
			decl.Model.Doc = doc 
			// 递归处理子字段
			for j := range decl.Model.Properties {
				p := &decl.Model.Properties[j]
				p.Doc = findImmediateComment(p.Pos.Line, comments)
			}
		}
		if decl.Group != nil { 
			decl.Group.Doc = doc 
			for j := range decl.Group.Endpoints {
				ep := &decl.Group.Endpoints[j]
				ep.Doc = findImmediateComment(ep.Pos.Line, comments)
				for k := range ep.Args {
					a := &ep.Args[k]
					a.Doc = findImmediateComment(a.Pos.Line, comments)
				}
			}
		}
	}
}

func findImmediateComment(line int, comments map[int]string) string {
	// 目前简单处理：取上一行内容
	if txt, ok := comments[line-1]; ok {
		return txt
	}
    // 或者同一行末尾？Lexer 虽然能分出来，但 collectComments 目前只抓整行注释
	return ""
}

// Validate 执行全局唯一性校验：模型重名、路由冲突、以及引用完整性校验
func (s *Schema) Validate() error {
	models := make(map[string]bool)
	modules := make(map[string]bool)
	decorators := make(map[string]bool)
	routes := make(map[string]string) 
	
	// 预置基础类型
	baseTypes := map[string]bool{
		"String": true, "Int": true, "Float": true, "Boolean": true, 
		"Time": true, "File": true, "Any": true, "Field": true,
	}

	// 第一遍：收集所有定义的名称
	for _, decl := range s.Declarations {
		if decl.Module != nil {
			if modules[decl.Module.Name] {
				return fmt.Errorf("%s: duplicate module defined: %s", decl.Module.Pos, decl.Module.Name)
			}
			modules[decl.Module.Name] = true
		}
		if decl.Model != nil {
			if models[decl.Model.Name] {
				return fmt.Errorf("%s: duplicate model defined: %s", decl.Model.Pos, decl.Model.Name)
			}
			models[decl.Model.Name] = true
		}
		if decl.Decorator != nil {
			if decorators[decl.Decorator.Name] {
				return fmt.Errorf("%s: duplicate decorator/validator defined: %s", decl.Decorator.Pos, decl.Decorator.Name)
			}
			decorators[decl.Decorator.Name] = true
		}
	}

	// 第二遍：验证引用完整性
	for _, decl := range s.Declarations {
		if decl.Model != nil {
			// 收集泛型参数
			localTypes := make(map[string]bool)
			for _, tp := range decl.Model.TypeParams {
				localTypes[tp] = true
			}
			
			for _, prop := range decl.Model.Properties {
				if err := validateTypeRef(prop.Type, models, baseTypes, localTypes); err != nil {
					return fmt.Errorf("%s: model %s property %s: %v", prop.Pos, decl.Model.Name, prop.Name, err)
				}
			}
		}
		if decl.Group != nil {
			for _, ep := range decl.Group.Endpoints {
				// 路由冲突校验
				fullPath := decl.Group.Path + ep.Path
				routeKey := fmt.Sprintf("%s %s", strings.ToUpper(ep.Method), fullPath)
				if existing, ok := routes[routeKey]; ok {
					return fmt.Errorf("%s: route conflict: %s is defined by both %s and %s", ep.Pos, routeKey, existing, ep.Name)
				}
				routes[routeKey] = ep.Name

				// 返回类型校验
				if err := validateTypeRef(ep.ReturnType, models, baseTypes, nil); err != nil {
					return fmt.Errorf("%s: endpoint %s: return type %v", ep.Pos, ep.Name, err)
				}

				// 参数类型校验
				for _, arg := range ep.Args {
					if err := validateTypeRef(arg.Type, models, baseTypes, nil); err != nil {
						return fmt.Errorf("%s: endpoint %s: arg %s %v", arg.Pos, ep.Name, arg.Name, err)
					}
				}
			}
		}
	}
	return nil
}

func (s *Schema) String() string {
	return fmt.Sprintf("Schema with %d declarations", len(s.Declarations))
}

func validateTypeRef(t TypeRef, models, baseTypes, localTypes map[string]bool) error {
	name := t.Name
	if !baseTypes[name] && !models[name] && (localTypes == nil || !localTypes[name]) {
		return fmt.Errorf("undefined type: %s", name)
	}
	for _, arg := range t.TypeArgs {
		if err := validateTypeRef(arg, models, baseTypes, localTypes); err != nil {
			return err
		}
	}
	return nil
}

var (
	resgenLexer = lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Comment", Pattern: `#[^\n]*`},
		{Name: "Whitespace", Pattern: `\s+`},
		{Name: "RoutePath", Pattern: `/[a-zA-Z0-9_\-\/:]*`}, 
		{Name: "String", Pattern: `"(?:\\.|[^"])*"`},
		{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_\.]*`},
		{Name: "Float", Pattern: `[-+]?\d*\.\d+`},
		{Name: "Int", Pattern: `[-+]?\d+`},
		{Name: "Arrow", Pattern: `=>`},
		{Name: "Punct", Pattern: `[-[!@#$%^&*()+_={}\|:;"'<,>.?/]|]`},
	})

	Parser = participle.MustBuild[Schema](
		participle.Lexer(resgenLexer),
		participle.Elide("Whitespace", "Comment"),
		participle.Unquote("String"), 
	)
)

// ParseFile 读取给定的 resgen 源文件并转换为 AST 树
func ParseFile(filename string) (*Schema, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Parser.Parse(filename, file)
}
