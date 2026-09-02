package lsp

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"github.com/xslasd/resgen/internal/formatter"
	"github.com/xslasd/resgen/internal/parser"
)

var (
	// files 存储在内存中的虚拟文件缓存
	filesMu sync.RWMutex
	files   = make(map[string]string)
)

type SymbolInfo struct {
	Name     string
	Kind     string
	Filename string
	Line     int
	Column   int
}

func initLog() {}

func writeLog(format string, a ...any) {}

func RunServer(version string) {
	initLog()
	writeLog("RunServer started with version: %s", version)

	handler := protocol.Handler{}

	handler.Initialize = func(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
		writeLog("Initialize called. Client name: %s", params.ClientInfo.Name)
		capabilities := handler.CreateServerCapabilities()
		capabilities.DocumentFormattingProvider = true
		capabilities.DefinitionProvider = true
		capabilities.HoverProvider = true
		capabilities.CompletionProvider = &protocol.CompletionOptions{
			TriggerCharacters: []string{"@", ":"},
		}
		capabilities.TextDocumentSync = protocol.TextDocumentSyncKindFull

		return protocol.InitializeResult{
			Capabilities: capabilities,
			ServerInfo: &protocol.InitializeResultServerInfo{
				Name:    "resgen-lsp",
				Version: &version,
			},
		}, nil
	}

	handler.TextDocumentHover = func(context *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
		filesMu.RLock()
		content, ok := files[params.TextDocument.URI]
		filesMu.RUnlock()
		if !ok {
			return nil, nil
		}

		filename := uriToPath(params.TextDocument.URI)
		schema, err := parser.ParseFileContent(filename, content)
		if err != nil {
			return nil, nil
		}

		targetLine := int(params.Position.Line) + 1
		targetCol := int(params.Position.Character) + 1

		ident := findIdentifierAt(schema, targetLine, targetCol)
		if ident == "" {
			return nil, nil
		}

		var hoverText string
		switch strings.ToLower(ident) {
		case "alias":
			hoverText = "### Built-in Directive: `@alias`\n\n用于指定模型字段或接口入参在传输层（JSON/Form/Query/Path/Header）使用的自定义别名，方便兼容老系统或不规范的接口命名规范。\n\n**示例**:\n```res\ninput QueryInput {\n    startTime: IntTime @alias(\"st_time\")\n}\n```"
		case "path":
			hoverText = "### Built-in Directive: `@path`\n\n标记参数来源于 URL 路径变量 (Path Variable)。"
		case "query":
			hoverText = "### Built-in Directive: `@query`\n\n标记参数来源于 URL 查询字符串 (Query String)。"
		case "header":
			hoverText = "### Built-in Directive: `@header`\n\n标记参数来源于 HTTP 请求头 (Header)。"
		case "required":
			hoverText = "### Built-in Directive: `@required`\n\n标记字段或形参必填且非空。"
		case "custombind":
			hoverText = "### Built-in Directive: `@customBind`\n\n接管参数绑定逻辑，由业务层在 Resolver 中手动实现 Bind 方法。"
		case "customvalidate":
			hoverText = "### Built-in Directive: `@customValidate`\n\n接管参数校验逻辑，由业务层在 Resolver 中手动实现 Validate 方法。"
		}

		if hoverText != "" {
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.MarkupKindMarkdown,
					Value: hoverText,
				},
			}, nil
		}

		return nil, nil
	}

	handler.TextDocumentDidOpen = func(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
		filesMu.Lock()
		files[params.TextDocument.URI] = params.TextDocument.Text
		filesMu.Unlock()
		
		go publishDiagnostics(context, params.TextDocument.URI, params.TextDocument.Text)
		return nil
	}

	handler.TextDocumentDidChange = func(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
		filesMu.Lock()
		var text string
		if len(params.ContentChanges) > 0 {
			switch change := params.ContentChanges[0].(type) {
			case protocol.TextDocumentContentChangeEventWhole:
				files[params.TextDocument.URI] = change.Text
				text = change.Text
			case protocol.TextDocumentContentChangeEvent:
				files[params.TextDocument.URI] = change.Text
				text = change.Text
			case map[string]any:
				if t, ok := change["text"].(string); ok {
					files[params.TextDocument.URI] = t
					text = t
				}
			}
		}
		filesMu.Unlock()
		
		if text != "" {
			go publishDiagnostics(context, params.TextDocument.URI, text)
		}
		return nil
	}

	handler.TextDocumentDidClose = func(context *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
		filesMu.Lock()
		defer filesMu.Unlock()
		delete(files, params.TextDocument.URI)
		return nil
	}

	handler.TextDocumentFormatting = func(context *glsp.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
		writeLog("TextDocumentFormatting called for URI: %s", params.TextDocument.URI)
		filesMu.RLock()
		content, ok := files[params.TextDocument.URI]
		filesMu.RUnlock()
		if !ok {
			writeLog("TextDocumentFormatting error: file not found in memory: %s", params.TextDocument.URI)
			return nil, fmt.Errorf("file not found in memory: %s", params.TextDocument.URI)
		}

		filename := uriToPath(params.TextDocument.URI)
		schema, err := parser.ParseFileContent(filename, content)
		if err != nil {
			return nil, err
		}

		tabSize := 4
		if val, ok := params.Options["tabSize"]; ok {
			if f, ok := val.(float64); ok {
				tabSize = int(f)
			} else if i, ok := val.(int); ok {
				tabSize = i
			}
		}

		for i := range schema.Declarations {
			decl := &schema.Declarations[i]
			if decl.Model != nil && (decl.Model.Keyword == "type" || decl.Model.Keyword == "input" || decl.Model.Keyword == "wrap") {
				for j := range decl.Model.Properties {
					decl.Model.Properties[j].Name = camelToSnake(decl.Model.Properties[j].Name)
				}
			}
		}

		var sb strings.Builder
		f := formatter.NewFormatter(tabSize)
		if err := f.Format(schema, &sb); err != nil {
			return nil, err
		}

		formatted := sb.String()

		lines := strings.Split(content, "\n")
		lineCount := len(lines)
		lastLineLen := 0
		if lineCount > 0 {
			lastLineLen = len(lines[lineCount-1])
		}

		return []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: uint32(lineCount), Character: uint32(lastLineLen)},
				},
				NewText: formatted,
			},
		}, nil
	}

	handler.TextDocumentDefinition = func(context *glsp.Context, params *protocol.DefinitionParams) (any, error) {
		filesMu.RLock()
		content, ok := files[params.TextDocument.URI]
		filesMu.RUnlock()
		if !ok {
			return nil, nil
		}

		filename := uriToPath(params.TextDocument.URI)
		schema, err := parser.ParseFileContent(filename, content)
		if err != nil {
			return nil, nil
		}

		targetLine := int(params.Position.Line) + 1
		targetCol := int(params.Position.Character) + 1

		ident := findIdentifierAt(schema, targetLine, targetCol)
		if ident == "" {
			return nil, nil
		}

		symbols := buildSymbolTable(filename)
		sym, found := symbols[ident]
		if !found {
			return nil, nil
		}

		return protocol.Location{
			URI: pathToURI(sym.Filename),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1), Character: uint32(sym.Column - 1)},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: uint32(sym.Column - 1 + len(sym.Name))},
			},
		}, nil
	}

	handler.TextDocumentCompletion = func(context *glsp.Context, params *protocol.CompletionParams) (any, error) {
		filesMu.RLock()
		content, ok := files[params.TextDocument.URI]
		filesMu.RUnlock()
		if !ok {
			filename := uriToPath(params.TextDocument.URI)
			data, err := os.ReadFile(filename)
			if err == nil {
				content = string(data)
			}
		}

		filename := uriToPath(params.TextDocument.URI)
		line := int(params.Position.Line)
		col := int(params.Position.Character)

		items := buildCompletions(filename, content, line, col)
		return items, nil
	}

	s := server.NewServer(&handler, "resgen-lsp", false)
	s.RunStdio()
}

func camelToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := rune(s[i-1])
				if prev >= 'a' && prev <= 'z' {
					result = append(result, '_')
				} else if i+1 < len(s) {
					next := rune(s[i+1])
					if next >= 'a' && next <= 'z' {
						result = append(result, '_')
					}
				}
			}
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	if u.Scheme != "file" {
		return uri
	}
	path := u.Path
	if len(path) > 0 && path[0] == '/' {
		if len(path) > 2 && path[2] == ':' {
			path = path[1:]
		}
	}
	return filepath.Clean(path)
}

func pathToURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}

func buildSymbolTable(currentFile string) map[string]SymbolInfo {
	symbols := make(map[string]SymbolInfo)
	dir := filepath.Dir(currentFile)

	// 1. 扫描内存中的文件（如果在同一个目录下）
	filesMu.RLock()
	for cachedURI, cachedContent := range files {
		filePath := uriToPath(cachedURI)
		if filepath.Dir(filePath) == dir {
			schema, err := parser.ParseFileContent(filePath, cachedContent)
			if err == nil {
				addSymbols(schema, filePath, symbols)
			}
		}
	}
	filesMu.RUnlock()

	// 2. 扫描磁盘上的文件（排除内存中已有的，避免重复解析）
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			ext := filepath.Ext(entry.Name())
			if entry.IsDir() || (ext != ".res" && ext != ".resgen") {
				continue
			}
			filePath := filepath.Join(dir, entry.Name())
			
			isCached := false
			filesMu.RLock()
			for cachedURI := range files {
				if uriToPath(cachedURI) == filePath {
					isCached = true
					break
				}
			}
			filesMu.RUnlock()

			if isCached {
				continue
			}

			schema, err := parser.ParseFile(filePath)
			if err == nil {
				addSymbols(schema, filePath, symbols)
			}
		}
	}

	return symbols
}

func addSymbols(schema *parser.Schema, filePath string, symbols map[string]SymbolInfo) {
	for _, decl := range schema.Declarations {
		if decl.Model != nil {
			symbols[decl.Model.Name] = SymbolInfo{
				Name:     decl.Model.Name,
				Kind:     "Model",
				Filename: filePath,
				Line:     decl.Model.Pos.Line,
				Column:   decl.Model.Pos.Column,
			}
		}
		if decl.Scalar != nil {
			symbols[decl.Scalar.Name] = SymbolInfo{
				Name:     decl.Scalar.Name,
				Kind:     "Scalar",
				Filename: filePath,
				Line:     decl.Scalar.Pos.Line,
				Column:   decl.Scalar.Pos.Column,
			}
		}
		if decl.Decorator != nil {
			symbols[decl.Decorator.Name] = SymbolInfo{
				Name:     decl.Decorator.Name,
				Kind:     "Decorator",
				Filename: filePath,
				Line:     decl.Decorator.Pos.Line,
				Column:   decl.Decorator.Pos.Column,
			}
		}
		if decl.Enum != nil {
			symbols[decl.Enum.Name] = SymbolInfo{
				Name:     decl.Enum.Name,
				Kind:     "Enum",
				Filename: filePath,
				Line:     decl.Enum.Pos.Line,
				Column:   decl.Enum.Pos.Column,
			}
		}
		if decl.Union != nil {
			symbols[decl.Union.Name] = SymbolInfo{
				Name:     decl.Union.Name,
				Kind:     "Union",
				Filename: filePath,
				Line:     decl.Union.Pos.Line,
				Column:   decl.Union.Pos.Column,
			}
		}
	}
}

func findIdentifierAt(schema *parser.Schema, line, col int) string {
	var foundIdent string

	inRange := func(pos lexer.Position, name string) bool {
		if pos.Line != line {
			return false
		}
		return col >= pos.Column && col <= pos.Column+len(name)
	}

	var visitTypeRef func(t parser.TypeRef) bool
	visitTypeRef = func(t parser.TypeRef) bool {
		if inRange(t.Pos, t.Name) {
			foundIdent = t.Name
			return true
		}
		for _, arg := range t.TypeArgs {
			if visitTypeRef(arg) {
				return true
			}
		}
		return false
	}

	visitDirective := func(d parser.DirectiveUsage) bool {
		if col >= d.Pos.Column && col <= d.Pos.Column+len(d.Name)+1 {
			foundIdent = d.Name
			return true
		}
		return false
	}

	for _, decl := range schema.Declarations {
		if decl.Model != nil {
			m := decl.Model
			for _, prop := range m.Properties {
				if visitTypeRef(prop.Type) {
					return foundIdent
				}
				for _, dir := range prop.Directives {
					if visitDirective(dir) {
						return foundIdent
					}
				}
			}
			for _, dir := range m.Directives {
				if visitDirective(dir) {
					return foundIdent
				}
			}
		}
		if decl.Enum != nil {
			if inRange(decl.Enum.Pos, decl.Enum.Name) {
				return decl.Enum.Name
			}
		}
		if decl.Union != nil {
			if inRange(decl.Union.Pos, decl.Union.Name) {
				return decl.Union.Name
			}
			for _, c := range decl.Union.Cases {
				if inRange(c.Pos, c.Type) {
					return c.Type
				}
			}
		}
		if decl.Group != nil {
			g := decl.Group
			for _, dir := range g.Directives {
				if visitDirective(dir) {
					return foundIdent
				}
			}
			for _, ep := range g.Endpoints {
				for _, dir := range ep.Directives {
					if visitDirective(dir) {
						return foundIdent
					}
				}
				if ep.ReturnType != nil && visitTypeRef(*ep.ReturnType) {
					return foundIdent
				}
				for _, arg := range ep.Args {
					for _, dir := range arg.Directives {
						if visitDirective(dir) {
							return foundIdent
						}
					}
					if visitTypeRef(arg.Type) {
						return foundIdent
					}
				}
			}
		}
	}

	return foundIdent
}

func publishDiagnostics(context *glsp.Context, uri, content string) {
	filename := uriToPath(uri)
	schema, err := parser.ParseFileContent(filename, content)
	var diagnostics []protocol.Diagnostic

	if err != nil {
		errStr := err.Error()
		re := regexp.MustCompile(`^.*?:(\d+):(\d+):\s*(.*)`)
		matches := re.FindStringSubmatch(errStr)

		var line, col uint32
		msg := errStr
		if len(matches) == 4 {
			if l, e := strconv.ParseUint(matches[1], 10, 32); e == nil {
				line = uint32(l)
			}
			if c, e := strconv.ParseUint(matches[2], 10, 32); e == nil {
				col = uint32(c)
			}
			msg = matches[3]
		}
		
		if line > 0 {
			line-- 
		}
		if col > 0 {
			col--
		}

		severity := protocol.DiagnosticSeverityError
		source := "resgen"
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: line, Character: col},
				End:   protocol.Position{Line: line, Character: col + 1},
			},
			Severity: &severity,
			Source:   &source,
			Message:  msg,
		})
	} else if schema != nil {
		severity := protocol.DiagnosticSeverityError
		source := "resgen"
		
		modelMap := make(map[string]*parser.ModelDecl)
		for _, decl := range schema.Declarations {
			if decl.Model != nil {
				modelMap[decl.Model.Name] = decl.Model
			}
		}

		var checkHasFile func(t parser.TypeRef) bool
		checkHasFile = func(t parser.TypeRef) bool {
			if t.Name == "File" {
				return true
			}
			if m, ok := modelMap[t.Name]; ok {
				for _, prop := range m.Properties {
					if checkHasFile(prop.Type) {
						return true
					}
				}
			}
			for _, arg := range t.TypeArgs {
				if checkHasFile(arg) {
					return true
				}
			}
			return false
		}

		// 校验指令使用（如 @alias 参数必须有效）
		checkDirectives := func(dirs []parser.DirectiveUsage) {
			for _, d := range dirs {
				if strings.EqualFold(d.Name, "alias") {
					aliasVal := ""
					if len(d.Args) > 0 {
						arg := d.Args[0]
						if arg.Value.String != nil {
							aliasVal = *arg.Value.String
						} else if arg.Value.Ident != nil {
							aliasVal = *arg.Value.Ident
						} else {
							diagnostics = append(diagnostics, protocol.Diagnostic{
								Range: protocol.Range{
									Start: protocol.Position{Line: uint32(d.Pos.Line - 1), Character: uint32(d.Pos.Column - 1)},
									End:   protocol.Position{Line: uint32(d.Pos.Line - 1), Character: uint32(d.Pos.Column + len(d.Name) + 1)},
								},
								Severity: &severity,
								Source:   &source,
								Message:  "语义错误：@alias 指令的参数必须是字符串或标识符，例如 @alias(\"st_time\")",
							})
							continue
						}
					}
					for _, m := range d.Meta {
						if strings.EqualFold(m.Key, "name") || strings.EqualFold(m.Key, "alias") {
							aliasVal = m.Value.MetaStr()
						}
					}
					if strings.TrimSpace(aliasVal) == "" {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(d.Pos.Line - 1), Character: uint32(d.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(d.Pos.Line - 1), Character: uint32(d.Pos.Column + len(d.Name) + 1)},
							},
							Severity: &severity,
							Source:   &source,
							Message:  "语义错误：@alias 指令必须包含一个非空的别名参数，例如 @alias(\"st_time\")",
						})
					}
				}
			}
		}

		for _, decl := range schema.Declarations {
			if decl.Model != nil {
				for _, prop := range decl.Model.Properties {
					checkDirectives(prop.Directives)
				}
			}
			if decl.Group != nil {
				for _, ep := range decl.Group.Endpoints {
					for _, arg := range ep.Args {
						checkDirectives(arg.Directives)
					}
				}
			}
			if decl.Scalar != nil {
				if decl.Scalar.Base == "File" {
					diagnostics = append(diagnostics, protocol.Diagnostic{
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(decl.Scalar.Pos.Line - 1), Character: uint32(decl.Scalar.Pos.Column - 1)},
							End:   protocol.Position{Line: uint32(decl.Scalar.Pos.Line - 1), Character: uint32(decl.Scalar.Pos.Column + len(decl.Scalar.Name))},
						},
						Severity: &severity,
						Source:   &source,
						Message:  fmt.Sprintf("语义错误：自定义标量 '%s' 不能继承自 File 类型", decl.Scalar.Name),
					})
				}
			}
			if decl.Union != nil {
				for _, c := range decl.Union.Cases {
					if c.Type == "File" {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(c.Pos.Line - 1), Character: uint32(c.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(c.Pos.Line - 1), Character: uint32(c.Pos.Column + len(c.Type))},
							},
							Severity: &severity,
							Source:   &source,
							Message:  fmt.Sprintf("语义错误：联合类型分支 '%s' 不能使用 File 类型", c.Key),
						})
					}
				}
				symbols := buildSymbolTable(filename)
				sym, ok := symbols[decl.Union.ParamName]
				if !ok || sym.Kind != "Enum" {
					diagnostics = append(diagnostics, protocol.Diagnostic{
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(decl.Union.Pos.Line - 1), Character: uint32(decl.Union.Pos.Column - 1)},
							End:   protocol.Position{Line: uint32(decl.Union.Pos.Line - 1), Character: uint32(decl.Union.Pos.Column + len(decl.Union.Name))},
						},
						Severity: &severity,
						Source:   &source,
						Message:  fmt.Sprintf("语义错误：联合类型 '%s' 的判别器 '%s' 无效，必须是一个枚举 (Enum) 类型", decl.Union.Name, decl.Union.ParamName),
					})
				}
			}

			if decl.Model != nil {
				for _, prop := range decl.Model.Properties {
					if prop.Type.Name == "Field" {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(prop.Pos.Line - 1), Character: uint32(prop.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(prop.Pos.Line - 1), Character: uint32(prop.Pos.Column + len(prop.Name))},
							},
							Severity: &severity,
							Source:   &source,
							Message:  fmt.Sprintf("语义错误：模型属性 '%s.%s' 不能使用 'Field' 类型。'Field' 是专属于校验器形参的字段引用元类型，绝对不能作为普通属性类型！若需表达动态或任意结构数据，请选用 'Any' 类型", decl.Model.Name, prop.Name),
						})
					}
					if decl.Model.Keyword != "input" && prop.Type.Name == "File" {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(prop.Pos.Line - 1), Character: uint32(prop.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(prop.Pos.Line - 1), Character: uint32(prop.Pos.Column + len(prop.Name))},
							},
							Severity: &severity,
							Source:   &source,
							Message:  fmt.Sprintf("语义错误：输出模型 '%s.%s' 不能包含 'File' 属性字段。文件流无法在 JSON/XML 结构体内部嵌套序列化返回；若接口需返回/下载文件，请将接口的返回值直接声明为 'File' (例如 => Download(): File [ctype=stream])", decl.Model.Name, prop.Name),
						})
					}
				}
			}
			if decl.Group != nil {
				for _, ep := range decl.Group.Endpoints {
					if len(ep.Args) > 1 {
						for _, arg := range ep.Args {
							if arg.Name == "" {
								diagnostics = append(diagnostics, protocol.Diagnostic{
									Range: protocol.Range{
										Start: protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1)},
										End:   protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column + len(arg.Type.Name))},
									},
									Severity: &severity,
									Source:   &source,
									Message:  fmt.Sprintf("端点 %s 语义错误: 匿名参数 (顶级 Payload) 只能作为唯一参数使用，不可与其他参数混用", ep.Name),
								})
								break
							}
						}
					}

					if ep.ReturnType != nil {
						var checkFileArray func(t parser.TypeRef) bool
						checkFileArray = func(t parser.TypeRef) bool {
							if t.Name == "File" && t.IsArray {
								return true
							}
							for _, arg := range t.TypeArgs {
								if checkFileArray(arg) {
									return true
								}
							}
							return false
						}
						if checkFileArray(*ep.ReturnType) {
							diagnostics = append(diagnostics, protocol.Diagnostic{
								Range: protocol.Range{
									Start: protocol.Position{Line: uint32(ep.ReturnType.Pos.Line - 1), Character: uint32(ep.ReturnType.Pos.Column - 1)},
									End:   protocol.Position{Line: uint32(ep.ReturnType.Pos.Line - 1), Character: uint32(ep.ReturnType.Pos.Column + len(ep.ReturnType.Name) + 2)},
								},
								Severity: &severity,
								Source:   &source,
								Message:  fmt.Sprintf("语义错误：接口 [%s %s] 的出参不能声明为文件数组 '[File]'。单个 HTTP 响应无法承载多个独立文件流。推荐方案：1) 打包为 ZIP 压缩流返回 (返回类型设为 File [ctype=zip])；2) 或返回包含下载地址的文件列表结构 (例如 ResData<[FileItem]>)。", ep.Method, ep.Path),
							})
						}
					}

					hasFile := false
					for _, arg := range ep.Args {
						if checkHasFile(arg.Type) {
							hasFile = true
							break
						}
					}
					if hasFile {
						ctype := ""
						for _, meta := range ep.RequestMeta {
							if strings.ToLower(meta.Key) == "ctype" && meta.Value.Str != nil {
								ctype = strings.ToLower(*meta.Value.Str)
							}
						}
						if ctype == "" {
							for _, meta := range ep.ResponseMeta {
								if strings.ToLower(meta.Key) == "ctype" && meta.Value.Str != nil {
									ctype = strings.ToLower(*meta.Value.Str)
								}
							}
						}
						isJsonCtype := ctype == "json" || ctype == "application/json"
						isFormCtype := ctype == "form" || ctype == "application/x-www-form-urlencoded"
						
						if isJsonCtype {
							diagnostics = append(diagnostics, protocol.Diagnostic{
								Range: protocol.Range{
									Start: protocol.Position{Line: uint32(ep.Pos.Line - 1), Character: uint32(ep.Pos.Column - 1)},
									End:   protocol.Position{Line: uint32(ep.Pos.Line - 1), Character: uint32(ep.Pos.Column + len(ep.Name))},
								},
								Severity: &severity,
								Source:   &source,
								Message:  fmt.Sprintf("语义错误：接口 [%s %s] 的 input 中包含 File 字段，不能与 ctype=json 同时使用。File 字段必须通过 multipart/form-data 传输，请将 ctype 改为 multipart", ep.Method, ep.Path),
							})
						} else if isFormCtype {
							diagnostics = append(diagnostics, protocol.Diagnostic{
								Range: protocol.Range{
									Start: protocol.Position{Line: uint32(ep.Pos.Line - 1), Character: uint32(ep.Pos.Column - 1)},
									End:   protocol.Position{Line: uint32(ep.Pos.Line - 1), Character: uint32(ep.Pos.Column + len(ep.Name))},
								},
								Severity: &severity,
								Source:   &source,
								Message:  fmt.Sprintf("语义错误：接口 [%s %s] 的 input 中包含 File 字段，不能与 ctype=form（application/x-www-form-urlencoded）同时使用。文件上传必须使用 multipart/form-data，请将 ctype 改为 multipart", ep.Method, ep.Path),
							})
						}
					}
				}
			}
		}
	}

	context.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

type directiveDef struct {
	name       string
	insertText string
	isSnippet  bool
	detail     string
	doc        string
}

var builtinDirectives = []directiveDef{
	{name: "path", insertText: "path", detail: "@path - URL 路径变量", doc: "标记参数来源于 URL 路径变量 (Path Variable)"},
	{name: "query", insertText: "query", detail: "@query - URL 查询参数", doc: "标记参数来源于 URL 查询字符串 (Query String)"},
	{name: "header", insertText: "header", detail: "@header - HTTP 请求头", doc: "标记参数来源于 HTTP 请求头 (Header)"},
	{name: "alias", insertText: "alias(\"${1:field_name}\")", isSnippet: true, detail: "@alias(\"...\") - 传输层字段别名", doc: "用于指定模型字段或接口入参在传输层使用的自定义别名"},
	{name: "required", insertText: "required", detail: "@required - 必填校验", doc: "标记字段或形参必填且非空"},
	{name: "customBind", insertText: "customBind", detail: "@customBind - 自定义绑定", doc: "接管参数绑定逻辑，由业务层在 Resolver 中手动实现 Bind 方法"},
	{name: "customValidate", insertText: "customValidate", detail: "@customValidate - 自定义校验", doc: "接管参数校验逻辑，由业务层在 Resolver 中手动实现 Validate 方法"},
}

var builtinTypes = []struct {
	name   string
	detail string
	doc    string
}{
	{"String", "内置标量类型 (string)", "字符串类型"},
	{"Int", "内置标量类型 (int64)", "64 位整数类型"},
	{"Float", "内置标量类型 (float64)", "64 位浮点数类型"},
	{"Boolean", "内置标量类型 (bool)", "布尔类型 (true/false)"},
	{"Time", "内置标量类型 (time.Time)", "时间日期类型"},
	{"File", "内置标量类型 (*multipart.FileHeader)", "文件上传类型"},
	{"Any", "内置标量类型 (interface{})", "任意对象类型"},
	{"Field", "内置标量类型", "动态字段修饰器"},
}

var builtinKeywords = []struct {
	label      string
	insertText string
	isSnippet  bool
	detail     string
	doc        string
}{
	{"module", "module ${1:ModuleName} {\n\t$0\n}", true, "声明一个功能模块", "module 用于组织相关的 HTTP 路由和模型"},
	{"type", "type ${1:Name} {\n\t${2:id}: ${3:Int} @alias(\"id\")\n\t$0\n}", true, "声明业务数据模型", "type 定义普通数据传输对象或实体"},
	{"input", "input ${1:Name} {\n\t${2:id}: ${3:Int} @query\n\t$0\n}", true, "声明请求入参模型", "input 定义接口的输入参数"},
	{"wrap", "wrap ${1:Name} {\n\tcode: Int\n\tmessage: String\n\tdata: Field\n}", true, "声明统一响应包装器", "wrap 定义全局或分组的响应包装层"},
	{"group", "group ${1:GroupName} {\n\t$0\n}", true, "声明接口路由分组", "group 包含一组具有相同前缀或中间件的 HTTP 接口"},
	{"scalar", "scalar ${1:Name}", false, "声明自定义标量", "scalar 扩展自定义基本类型"},
	{"union", "union ${1:Name} = ${2:TypeA} | ${3:TypeB}", true, "声明联合类型", "union 用于定义多态返回值"},
	{"enum", "enum ${1:Name} {\n\t$0\n}", true, "声明枚举类型", "enum 定义枚举常量"},
	{"GET", "GET /${1:path} (${2:input}) -> ${3:Output}", true, "HTTP GET 接口", "声明一个 GET 请求路由"},
	{"POST", "POST /${1:path} (${2:input}) -> ${3:Output}", true, "HTTP POST 接口", "声明一个 POST 请求路由"},
	{"PUT", "PUT /${1:path} (${2:input}) -> ${3:Output}", true, "HTTP PUT 接口", "声明一个 PUT 请求路由"},
	{"DELETE", "DELETE /${1:path} (${2:input}) -> ${3:Output}", true, "HTTP DELETE 接口", "声明一个 DELETE 请求路由"},
	{"PATCH", "PATCH /${1:path} (${2:input}) -> ${3:Output}", true, "HTTP PATCH 接口", "声明一个 PATCH 请求路由"},
}

func buildCompletions(filename, content string, line, col int) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	lines := strings.Split(content, "\n")
	prefix := ""
	if line < len(lines) {
		lineStr := strings.TrimRight(lines[line], "\r")
		if col <= len(lineStr) {
			prefix = lineStr[:col]
		} else {
			prefix = lineStr
		}
	}

	// 1. 判断是否正在输入指令/装饰器（光标紧随 @ 或正在输入 @xxx）
	isDirective := false
	atIdx := strings.LastIndex(prefix, "@")
	if atIdx != -1 {
		afterAt := prefix[atIdx+1:]
		if !strings.ContainsAny(afterAt, " \t(){}[],:\"") {
			isDirective = true
		}
	}

	if isDirective {
		directiveSnippetFormat := protocol.InsertTextFormatSnippet
		directivePlainFormat := protocol.InsertTextFormatPlainText
		directiveKind := protocol.CompletionItemKindProperty

		for _, d := range builtinDirectives {
			insertText := d.insertText
			format := directivePlainFormat
			if d.isSnippet {
				format = directiveSnippetFormat
			}

			detail := d.detail
			items = append(items, protocol.CompletionItem{
				Label:            "@" + d.name,
				Kind:             &directiveKind,
				Detail:           &detail,
				Documentation:    protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: d.doc},
				InsertText:       &insertText,
				InsertTextFormat: &format,
			})
		}

		// 自定义 decorator
		symbols := buildSymbolTable(filename)
		customKind := protocol.CompletionItemKindFunction
		for _, sym := range symbols {
			if sym.Kind == "Decorator" {
				name := sym.Name
				detail := fmt.Sprintf("@%s - 自定义装饰器", name)
				doc := fmt.Sprintf("定义于: %s:%d", filepath.Base(sym.Filename), sym.Line)
				items = append(items, protocol.CompletionItem{
					Label:         "@" + name,
					Kind:          &customKind,
					Detail:        &detail,
					Documentation: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc},
					InsertText:    &name,
				})
			}
		}
		return items
	}

	// 2. 判断是否在类型声明位置（在 ":" 后面）
	colonIdx := strings.LastIndex(prefix, ":")
	isTypePosition := false
	if colonIdx != -1 {
		afterColon := prefix[colonIdx+1:]
		if !strings.ContainsAny(afterColon, "{}(),;\"") {
			isTypePosition = true
		}
	}

	typeKind := protocol.CompletionItemKindClass
	// 内置标量类型
	for _, bt := range builtinTypes {
		name := bt.name
		detail := bt.detail
		doc := bt.doc
		items = append(items, protocol.CompletionItem{
			Label:         name,
			Kind:          &typeKind,
			Detail:        &detail,
			Documentation: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc},
			InsertText:    &name,
		})
	}

	// 自定义模型/类型符号
	symbols := buildSymbolTable(filename)
	for _, sym := range symbols {
		var k protocol.CompletionItemKind
		switch sym.Kind {
		case "Model":
			k = protocol.CompletionItemKindClass
		case "Enum":
			k = protocol.CompletionItemKindEnum
		case "Scalar":
			k = protocol.CompletionItemKindTypeParameter
		case "Union":
			k = protocol.CompletionItemKindInterface
		default:
			continue
		}
		name := sym.Name
		detail := fmt.Sprintf("%s (%s)", sym.Name, sym.Kind)
		doc := fmt.Sprintf("定义于: %s:%d", filepath.Base(sym.Filename), sym.Line)
		items = append(items, protocol.CompletionItem{
			Label:         name,
			Kind:          &k,
			Detail:        &detail,
			Documentation: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc},
			InsertText:    &name,
		})
	}

	// 若明确在类型赋值位置，则无需混入顶层结构关键字
	if isTypePosition {
		return items
	}

	// 3. 关键字与语句片段 (Snippets)
	snippetFormat := protocol.InsertTextFormatSnippet
	plainFormat := protocol.InsertTextFormatPlainText
	kwKind := protocol.CompletionItemKindKeyword
	snipKind := protocol.CompletionItemKindSnippet

	for _, kw := range builtinKeywords {
		label := kw.label
		insertText := kw.insertText
		detail := kw.detail
		doc := kw.doc
		kind := kwKind
		format := plainFormat
		if kw.isSnippet {
			kind = snipKind
			format = snippetFormat
		}
		items = append(items, protocol.CompletionItem{
			Label:            label,
			Kind:             &kind,
			Detail:           &detail,
			Documentation:    protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc},
			InsertText:       &insertText,
			InsertTextFormat: &format,
		})
	}

	// 4. 指令（通用候选）
	dirKind := protocol.CompletionItemKindProperty
	for _, d := range builtinDirectives {
		label := "@" + d.name
		insertText := "@" + d.insertText
		format := plainFormat
		if d.isSnippet {
			format = snippetFormat
		}
		detail := d.detail
		items = append(items, protocol.CompletionItem{
			Label:            label,
			Kind:             &dirKind,
			Detail:           &detail,
			Documentation:    protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: d.doc},
			InsertText:       &insertText,
			InsertTextFormat: &format,
		})
	}

	return items
}
