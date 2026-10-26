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
	"time"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"github.com/xslasd/resgen/internal/formatter"
	"github.com/xslasd/resgen/internal/parser"
)

type SymbolInfo struct {
	Name     string
	Kind     string // "Model", "Scalar", "Decorator", "Enum", "Union"
	Filename string
	Line     int
	Column   int
	Doc      string // 文档注释
	Detail   string // 完整定义代码预览
}

type fileCacheEntry struct {
	modTime time.Time
	symbols map[string]SymbolInfo
}

var (
	// files 存储在内存中的虚拟文件缓存
	filesMu sync.RWMutex
	files   = make(map[string]string)

	// symbolCache 磁盘文件符号缓存（基于文件修改时间 ModTime）
	cacheMu     sync.RWMutex
	symbolCache = make(map[string]fileCacheEntry)

	// 诊断发布防抖计时器
	diagMu     sync.Mutex
	diagTimers = make(map[string]*time.Timer)
)

func initLog() {}

func writeLog(format string, a ...any) {}

func RunServer(version string) {
	initLog()
	writeLog("RunServer started with version: %s", version)

	handler := NewHandler(version)
	s := server.NewServer(&handler, "resgen-lsp", false)
	s.RunStdio()
}

func NewHandler(version string) protocol.Handler {
	handler := protocol.Handler{}

	handler.Initialize = func(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
		writeLog("Initialize called. Client name: %s", params.ClientInfo.Name)
		capabilities := handler.CreateServerCapabilities()
		capabilities.DocumentFormattingProvider = true
		capabilities.DefinitionProvider = true
		capabilities.HoverProvider = true
		capabilities.CompletionProvider = &protocol.CompletionOptions{
			TriggerCharacters: []string{"@", ":", "[", "="},
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

		ident := findIdentifierAt(schema, targetLine, targetCol, content)
		if ident == "" {
			return nil, nil
		}

		var hoverText string
		switch strings.ToLower(ident) {
		case "alias":
			hoverText = "### Built-in Directive: `@alias`\n\n用于指定模型字段或接口入参在传输层（JSON/Form/Query/Path/Header）使用的自定义别名，方便兼容老系统或不规范的接口命名规范。\n\n**示例**:\n```res\ninput QueryInput {\n    startTime: IntTime @alias(\"st_time\")\n}\n```"
		case "path":
			hoverText = "### Built-in Directive: `@path`\n\n标记参数来源于 URL 路径变量 (Path Variable)。\n\n**示例**:\n```res\nGET /users/:id => GetUser(id: Int! @path): User\n```"
		case "query":
			hoverText = "### Built-in Directive: `@query`\n\n标记参数来源于 URL 查询字符串 (Query String)。\n\n**参数提取规则**:\n- **GET 接口**：未标注的平铺参数默认提取自 URL 查询字符串，通常无需显式标注 `@query`。\n- **POST / PUT / PATCH / DELETE 接口**：未标注的平铺形参默认绑定至 **请求体 (Body)** 并自动合成 `XxxArgs` 包装模型；若需要在非 GET 接口中强制提取自 URL Query，**必须显式标注 `@query`**。\n\n**示例**:\n```res\nPOST /tasks/trigger => TriggerTask(task_id: Int! @query): Boolean\nDELETE /tasks/item => DeleteItem(id: String! @query): Boolean\n```"
		case "header":
			hoverText = "### Built-in Directive: `@header`\n\n标记参数来源于 HTTP 请求头 (Header)。\n\n**示例**:\n```res\nGET /profile => GetProfile(token: String! @header): UserProfile\n```"
		case "required":
			hoverText = "### Built-in Directive: `@required`\n\n标记字段或形参必填且非空。"
		case "custombind":
			hoverText = "### Built-in Directive: `@customBind`\n\n接管参数绑定逻辑，由业务层在 Resolver 中手动实现 Bind 方法。"
		case "customvalidate":
			hoverText = "### Built-in Directive: `@customValidate`\n\n接管参数校验逻辑，由业务层在 Resolver 中手动实现 Validate 方法。"
		case "payload":
			hoverText = "### Built-in Directive: `@payload`\n\n标注载体字段在解密/解包后的内部明文结构。\n\n物理传输层为 String 密文或数据容器，在 DSL 中使用 `@payload(LoginReq)` 辅助说明其内部关联的明文数据模型（在 `seal` 信封与普通 API 模型中均可使用）。"
		case "seal":
			hoverText = "### Meta Attribute: `seal`\n\n在 RequestMeta 或 ResponseMeta 中声明该接口在物理传输层使用的加解封信封与实例化载荷。\n\n例如：`POST /login [seal=CipherWrap<LoginReq>] => Login(input: LoginReq): Token`"
		case "get":
			hoverText = "### HTTP Method: `GET`\n\n声明一个 HTTP GET 请求路由。\n\n**参数绑定规则**:\n- **默认规则**：未显式标注的平铺参数默认从 **URL Query**（查询字符串）提取。\n- **路径变量**：标注 `@path` 的参数从 URL 路径提取（如 `/users/:id`）。\n- **请求头**：标注 `@header` 的参数从 HTTP Header 提取。\n- **模型入参**：若传入自定义 `input` 模型，生成器会自动将其字段扁平化展开为 URL 查询参数（禁止嵌套子结构）。"
		case "post":
			hoverText = "### HTTP Method: `POST`\n\n声明一个 HTTP POST 请求路由。\n\n**参数绑定规则**:\n- **平铺参数自动合成**：未显式标注的平铺形参（无论是单标量如 `task_id: Int!` 还是多个参数）默认绑定至 **请求体 (Body)** 反序列化，编译器会自动合成对应的 `XxxArgs` 包装模型。\n- **单一输入模型**：若传入自定义 `input` 结构体模型作为唯一参数，直接映射为请求体 (Body) 载荷。\n- **强制 Query 参数**：若期望通过 URL 查询参数传递，必须显式声明 `@query`（例如 `task_id: Int! @query`）。\n- **路径变量**：标注 `@path` 从 URL 路径提取。\n- **请求头**：标注 `@header` 从 HTTP Header 提取。"
		case "put":
			hoverText = "### HTTP Method: `PUT`\n\n声明一个 HTTP PUT 请求路由。\n\n**参数绑定规则**:\n- **与 POST 一致**：未显式标注的平铺形参默认绑定至 **请求体 (Body)** 并自动合成 `XxxArgs` 包装模型。\n- **单一输入模型**：传入自定义 `input` 模型直接映射为请求体 (Body) 载荷。\n- **强制 Query 参数**：显式标注 `@query` 从 URL 查询参数提取。\n- **路径变量**：标注 `@path` 从 URL 路径提取。"
		case "patch":
			hoverText = "### HTTP Method: `PATCH`\n\n声明一个 HTTP PATCH 请求路由。\n\n**参数绑定规则**:\n- **与 POST 一致**：未显式标注的平铺形参默认绑定至 **请求体 (Body)** 并自动合成 `XxxArgs` 包装模型。\n- **单一输入模型**：传入自定义 `input` 模型直接映射为请求体 (Body) 增量更新载荷。\n- **强制 Query 参数**：显式标注 `@query` 从 URL 查询参数提取。\n- **路径变量**：标注 `@path` 从 URL 路径提取。"
		case "delete":
			hoverText = "### HTTP Method: `DELETE`\n\n声明一个 HTTP DELETE 请求路由。\n\n**参数绑定规则**:\n- **与 POST/PUT/PATCH 完全一致**：未显式标注的平铺形参默认绑定至 **请求体 (Body)** 并自动合成 `XxxArgs` 包装模型。\n- **特别提示**：若期望通过 URL 查询参数传递删除筛选条件，**必须显式声明 `@query`**（例如 `id: String! @query`）。\n- **路径变量**：标注 `@path` 从 URL 路径提取（例如 `DELETE /items/:id => DeleteItem(id: Int! @path): Boolean`）。"
		case "ctype":
			hoverText = "### Meta Attribute: `ctype`\n\n指定接口请求或响应的传输编码格式 (Content-Type)。\n\n**可选参数**:\n- `[ctype=stream]`：二进制数据流下载 (如文件导出)\n- `[ctype=zip]`：ZIP 压缩包文件下载流\n- `[ctype=multipart]`：文件上传表单 (`multipart/form-data`)\n- `[ctype=json]`：默认 JSON 格式\n- `[ctype=form]`：URL 编码表单 (`application/x-www-form-urlencoded`)"
		case "ws":
			hoverText = "### Meta Attribute: `ws=true`\n\n声明该端点为 WebSocket 全双工长连接端点，框架会自动生成协议升级（Upgrader）与双工通信连接处理器。"
		case "prefix":
			hoverText = "### Meta Attribute: `prefix=\"/api\"`\n\n为路由分组 (group) 或端点声明自定义统一路径前缀。"
		case "string":
			hoverText = "### 内置标量类型: `String`\n\n字符串标量类型。\n- `String!` 映射为 Go `string`（必填非空校验）；\n- `String` 映射为 Go `*string`（可选指针）。"
		case "int":
			hoverText = "### 内置标量类型: `Int`\n\n64 位整数标量类型。\n- `Int!` 映射为 Go `int64`；\n- `Int` 映射为 Go `*int64`。"
		case "float":
			hoverText = "### 内置标量类型: `Float`\n\n64 位浮点数标量类型。\n- `Float!` 映射为 Go `float64`；\n- `Float` 映射为 Go `*float64`。"
		case "boolean":
			hoverText = "### 内置标量类型: `Boolean`\n\n布尔标量类型。\n- `Boolean!` 映射为 Go `bool`；\n- `Boolean` 映射为 Go `*bool`。"
		case "time":
			hoverText = "### 内置标量类型: `Time`\n\n日期时间标量类型，映射为 Go `time.Time` / `*time.Time`。"
		case "file":
			hoverText = "### 内置标量类型: `File`\n\n文件数据传输标量：\n- **作为入参字段**：用于文件上传，映射为 `*multipart.FileHeader`，所属接口必须使用 `[ctype=multipart]`；\n- **作为出参返回值**：用于文件下载，映射为 `*LocalFileDownload` 强类型下载流载体；\n- **约束**：禁止在普通输出模型中嵌套包含 File 字段，禁止声明为文件数组 `[File]`。"
		case "any":
			hoverText = "### 内置动态标量: `Any`\n\n逃生舱类型，映射为 Go `any` (`interface{}`)。可承载任意形式的动态散列或未知结构数据。"
		case "field":
			hoverText = "### 校验器元类型: `Field`\n\n**专用于校验器 (Validator) 的字段引用参数**。代码生成时会自动升格为被引用的 Go 字段变量表达式（用于跨字段对比校验）。⚠️ 严禁作为普通业务模型的属性类型！"
		}

		if hoverText == "" {
			symbols := buildSymbolTable(filename)
			if sym, found := symbols[ident]; found {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("### %s: `%s`\n\n", sym.Kind, sym.Name))
				if sym.Doc != "" {
					sb.WriteString(sym.Doc + "\n\n")
				}
				if sym.Detail != "" {
					sb.WriteString("```res\n" + sym.Detail + "\n```\n\n")
				}
				sb.WriteString(fmt.Sprintf("*定义于: `%s:%d`*", filepath.Base(sym.Filename), sym.Line))
				hoverText = sb.String()
			}
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
			debouncePublishDiagnostics(context, params.TextDocument.URI, text)
		}
		return nil
	}

	handler.TextDocumentDidClose = func(context *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
		filesMu.Lock()
		delete(files, params.TextDocument.URI)
		filesMu.Unlock()

		diagMu.Lock()
		if t, ok := diagTimers[params.TextDocument.URI]; ok {
			t.Stop()
			delete(diagTimers, params.TextDocument.URI)
		}
		diagMu.Unlock()
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

		ident := findIdentifierAt(schema, targetLine, targetCol, content)
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

	return handler
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

func debouncePublishDiagnostics(context *glsp.Context, uri, content string) {
	diagMu.Lock()
	defer diagMu.Unlock()

	if t, ok := diagTimers[uri]; ok {
		t.Stop()
	}

	diagTimers[uri] = time.AfterFunc(150*time.Millisecond, func() {
		publishDiagnostics(context, uri, content)
	})
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
	if unescaped, err := url.PathUnescape(path); err == nil {
		path = unescaped
	}
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
	u := url.URL{
		Scheme: "file",
		Path:   path,
	}
	return u.String()
}

var fallbackDeclRegex = regexp.MustCompile(`(?m)^\s*(type|input|wrap|enum|scalar|union)\s+([A-Za-z0-9_]+)`)

func extractFallbackSymbols(content, filePath string, symbols map[string]SymbolInfo) {
	lines := strings.Split(content, "\n")
	for lineIdx, line := range lines {
		matches := fallbackDeclRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			kw := strings.ToLower(matches[1])
			name := matches[2]
			kind := "Model"
			switch kw {
			case "enum":
				kind = "Enum"
			case "scalar":
				kind = "Scalar"
			case "union":
				kind = "Union"
			}
			if _, exists := symbols[name]; !exists {
				symbols[name] = SymbolInfo{
					Name:     name,
					Kind:     kind,
					Filename: filePath,
					Line:     lineIdx + 1,
					Column:   strings.Index(line, name) + 1,
					Detail:   strings.TrimSpace(line),
				}
			}
		}
	}
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
			} else {
				extractFallbackSymbols(cachedContent, filePath, symbols)
			}
		}
	}
	filesMu.RUnlock()

	// 2. 扫描磁盘上的文件（排除内存中已有的，带 ModTime 缓存）
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

			fi, err := entry.Info()
			if err != nil {
				continue
			}
			modTime := fi.ModTime()

			cacheMu.RLock()
			cached, hit := symbolCache[filePath]
			cacheMu.RUnlock()

			if hit && cached.modTime.Equal(modTime) {
				for k, v := range cached.symbols {
					symbols[k] = v
				}
				continue
			}

			schema, err := parser.ParseFile(filePath)
			if err == nil {
				fileSymbols := make(map[string]SymbolInfo)
				addSymbols(schema, filePath, fileSymbols)

				cacheMu.Lock()
				symbolCache[filePath] = fileCacheEntry{
					modTime: modTime,
					symbols: fileSymbols,
				}
				cacheMu.Unlock()

				for k, v := range fileSymbols {
					symbols[k] = v
				}
			}
		}
	}

	return symbols
}

func formatTypeRefStr(t parser.TypeRef) string {
	res := t.Name
	if len(t.TypeArgs) > 0 {
		var args []string
		for _, a := range t.TypeArgs {
			args = append(args, formatTypeRefStr(a))
		}
		res += "<" + strings.Join(args, ", ") + ">"
	}
	if t.ItemNotNull {
		res += "!"
	}
	if t.IsArray {
		res = "[" + res + "]"
		if t.ArrNotNull {
			res += "!"
		}
	}
	return res
}

func addSymbols(schema *parser.Schema, filePath string, symbols map[string]SymbolInfo) {
	for _, decl := range schema.Declarations {
		if decl.Model != nil {
			var sb strings.Builder
			tpStr := ""
			if len(decl.Model.TypeParams) > 0 {
				tpStr = "<" + strings.Join(decl.Model.TypeParams, ", ") + ">"
			}
			sb.WriteString(fmt.Sprintf("%s %s%s {\n", decl.Model.Keyword, decl.Model.Name, tpStr))
			for _, p := range decl.Model.Properties {
				var dirs []string
				for _, d := range p.Directives {
					dirs = append(dirs, "@"+d.Name)
				}
				dirStr := ""
				if len(dirs) > 0 {
					dirStr = " " + strings.Join(dirs, " ")
				}
				sb.WriteString(fmt.Sprintf("    %s: %s%s\n", p.Name, formatTypeRefStr(p.Type), dirStr))
			}
			sb.WriteString("}")

			symbols[decl.Model.Name] = SymbolInfo{
				Name:     decl.Model.Name,
				Kind:     "Model",
				Filename: filePath,
				Line:     decl.Model.Pos.Line,
				Column:   decl.Model.Pos.Column,
				Doc:      decl.Model.Doc,
				Detail:   sb.String(),
			}
		}
		if decl.Scalar != nil {
			detail := fmt.Sprintf("scalar %s: %s", decl.Scalar.Name, decl.Scalar.Base)
			symbols[decl.Scalar.Name] = SymbolInfo{
				Name:     decl.Scalar.Name,
				Kind:     "Scalar",
				Filename: filePath,
				Line:     decl.Scalar.Pos.Line,
				Column:   decl.Scalar.Pos.Column,
				Doc:      decl.Scalar.Doc,
				Detail:   detail,
			}
		}
		if decl.Decorator != nil {
			kw := "decorator"
			if decl.Decorator.IsDec {
				kw = "validator"
			}
			detail := fmt.Sprintf("%s %s(...)", kw, decl.Decorator.Name)
			symbols[decl.Decorator.Name] = SymbolInfo{
				Name:     decl.Decorator.Name,
				Kind:     "Decorator",
				Filename: filePath,
				Line:     decl.Decorator.Pos.Line,
				Column:   decl.Decorator.Pos.Column,
				Doc:      decl.Decorator.Doc,
				Detail:   detail,
			}
		}
		if decl.Enum != nil {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("enum %s {\n", decl.Enum.Name))
			for _, c := range decl.Enum.Cases {
				sb.WriteString(fmt.Sprintf("    %s\n", c.Name))
			}
			sb.WriteString("}")

			symbols[decl.Enum.Name] = SymbolInfo{
				Name:     decl.Enum.Name,
				Kind:     "Enum",
				Filename: filePath,
				Line:     decl.Enum.Pos.Line,
				Column:   decl.Enum.Pos.Column,
				Doc:      decl.Enum.Doc,
				Detail:   sb.String(),
			}
		}
		if decl.Union != nil {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("union %s(%s) {\n", decl.Union.Name, decl.Union.ParamName))
			for _, c := range decl.Union.Cases {
				sb.WriteString(fmt.Sprintf("    %s: %s\n", c.Key, c.Type))
			}
			sb.WriteString("}")

			symbols[decl.Union.Name] = SymbolInfo{
				Name:     decl.Union.Name,
				Kind:     "Union",
				Filename: filePath,
				Line:     decl.Union.Pos.Line,
				Column:   decl.Union.Pos.Column,
				Doc:      decl.Union.Doc,
				Detail:   sb.String(),
			}
		}
	}
}

func findIdentifierAt(schema *parser.Schema, line, col int, content ...string) string {
	var fileContent string
	if len(content) > 0 {
		fileContent = content[0]
	}

	// 1. 如果提供了源码文本，优先尝试从光标位置提取词法单词
	word := getWordAt(fileContent, line, col)
	if word != "" {
		wLower := strings.ToLower(word)
		switch wLower {
		case "get", "post", "put", "delete", "patch",
			"alias", "path", "query", "header", "required",
			"custombind", "customvalidate", "payload", "seal",
			"ctype", "ws", "prefix":
			return word
		}
	}

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

	if schema != nil {
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
					if inRange(ep.Pos, ep.Method) {
						return ep.Method
					}
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
	}

	if foundIdent != "" {
		return foundIdent
	}

	return word
}

func getWordAt(content string, line, col int) string {
	if content == "" || line < 1 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if line > len(lines) {
		return ""
	}
	lineStr := strings.TrimRight(lines[line-1], "\r")
	if len(lineStr) == 0 {
		return ""
	}

	targetIdx := col - 1
	if targetIdx < 0 {
		targetIdx = 0
	}
	if targetIdx > len(lineStr) {
		targetIdx = len(lineStr)
	}

	// 若光标在行尾或非单词字符处，尝试向前回退一个字符（例如光标停在单词末尾紧随的位置）
	if targetIdx == len(lineStr) || !isWordChar(rune(lineStr[targetIdx])) {
		if targetIdx > 0 && isWordChar(rune(lineStr[targetIdx-1])) {
			targetIdx--
		} else {
			return ""
		}
	}

	start := targetIdx
	for start > 0 && isWordChar(rune(lineStr[start-1])) {
		start--
	}
	end := targetIdx
	for end < len(lineStr) && isWordChar(rune(lineStr[end])) {
		end++
	}

	word := lineStr[start:end]
	return strings.TrimPrefix(word, "@")
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '@'
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
		symbols := buildSymbolTable(filename)
		validTypes := make(map[string]bool)
		for _, bt := range builtinTypes {
			validTypes[bt.name] = true
		}
		for symName := range symbols {
			validTypes[symName] = true
		}

		for _, decl := range schema.Declarations {
			if decl.Model != nil {
				modelMap[decl.Model.Name] = decl.Model
				validTypes[decl.Model.Name] = true
			}
			if decl.Scalar != nil {
				validTypes[decl.Scalar.Name] = true
			}
			if decl.Enum != nil {
				validTypes[decl.Enum.Name] = true
			}
			if decl.Union != nil {
				validTypes[decl.Union.Name] = true
			}
		}

		checkTypeExists := func(t parser.TypeRef, localTypeParams map[string]bool) {
			var checkRef func(ref parser.TypeRef)
			checkRef = func(ref parser.TypeRef) {
				name := ref.Name
				if name != "" && !validTypes[name] && (localTypeParams == nil || !localTypeParams[name]) {
					diagnostics = append(diagnostics, protocol.Diagnostic{
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(ref.Pos.Line - 1), Character: uint32(ref.Pos.Column - 1)},
							End:   protocol.Position{Line: uint32(ref.Pos.Line - 1), Character: uint32(ref.Pos.Column - 1 + len(name))},
						},
						Severity: &severity,
						Source:   &source,
						Message:  fmt.Sprintf("未定义的类型: '%s'。请检查拼写或确保已声明对应的 Model/Scalar/Enum/Union", name),
					})
				}
				for _, arg := range ref.TypeArgs {
					checkRef(arg)
				}
			}
			checkRef(t)
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
				localTP := make(map[string]bool)
				for _, tp := range decl.Model.TypeParams {
					localTP[tp] = true
				}
				for _, prop := range decl.Model.Properties {
					checkDirectives(prop.Directives)
					checkTypeExists(prop.Type, localTP)
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

			if decl.Group != nil {
				for _, ep := range decl.Group.Endpoints {
					for _, arg := range ep.Args {
						checkDirectives(arg.Directives)
						if arg.Type.Name != "" {
							checkTypeExists(arg.Type, nil)
						}
					}
					if ep.ReturnType != nil && ep.ReturnType.Name != "" {
						checkTypeExists(*ep.ReturnType, nil)
					}

					// 检查 [seal=...] 信封模型声明有效性
					checkSeal := func(metas []parser.MetaEntry) {
						for _, meta := range metas {
							if strings.ToLower(meta.Key) == "seal" {
								sealName := ""
								if meta.Value.Generic != nil {
									sealName = meta.Value.Generic.Name
									for _, targ := range meta.Value.Generic.Args {
										checkTypeExists(targ, nil)
									}
								} else if meta.Value.Str != nil {
									sealName = *meta.Value.Str
								}
								if sealName != "" && !validTypes[sealName] {
									diagnostics = append(diagnostics, protocol.Diagnostic{
										Range: protocol.Range{
											Start: protocol.Position{Line: uint32(meta.Pos.Line - 1), Character: uint32(meta.Pos.Column - 1)},
											End:   protocol.Position{Line: uint32(meta.Pos.Line - 1), Character: uint32(meta.Pos.Column - 1 + len(meta.Key) + 1 + len(sealName))},
										},
										Severity: &severity,
										Source:   &source,
										Message:  fmt.Sprintf("未定义的信封模型: '%s'。加解密信封必须是一个已定义的模型", sealName),
									})
								}
							}
						}
					}
					checkSeal(ep.RequestMeta)
					checkSeal(ep.ResponseMeta)

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
	{name: "query", insertText: "query", detail: "@query - URL 查询参数", doc: "标记参数来源于 URL 查询字符串 (Query String)。在 POST/PUT/PATCH/DELETE 接口中，未标注的平铺参数默认绑定至请求体 (Body) 并自动合成 XxxArgs 包装模型；若需强制使用 URL Query 传参，必须显式标注 @query"},
	{name: "header", insertText: "header", detail: "@header - HTTP 请求头", doc: "标记参数来源于 HTTP 请求头 (Header)"},
	{name: "alias", insertText: "alias(\"${1:field_name}\")", isSnippet: true, detail: "@alias(\"...\") - 传输层字段别名", doc: "用于指定模型字段或接口入参在传输层使用的自定义别名"},
	{name: "required", insertText: "required", detail: "@required - 必填校验", doc: "标记字段或形参必填且非空"},
	{name: "customBind", insertText: "customBind", detail: "@customBind - 自定义绑定", doc: "接管参数绑定逻辑，由业务层在 Resolver 中手动实现 Bind 方法"},
	{name: "customValidate", insertText: "customValidate", detail: "@customValidate - 自定义校验", doc: "接管参数校验逻辑，由业务层在 Resolver 中手动实现 Validate 方法"},
	{name: "payload", insertText: "payload(${1:PayloadModel})", isSnippet: true, detail: "@payload(M) - 载体字段补充明文结构说明", doc: "标注字段解封后的内部明文结构。物理层为字符串，业务上补充说明解包后的明文模型（支持 seal 信封和普通模型）。"},
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
	{"module", "module ${1:ModuleName}\n\n$0", true, "声明一个功能模块", "module 用于组织相关的 HTTP 路由和模型"},
	{"type", "type ${1:Name} {\n\t${2:id}: ${3:Int} @alias(\"id\")\n\t$0\n}", true, "声明业务数据模型", "type 定义普通数据传输对象或实体"},
	{"input", "input ${1:Name} {\n\t${2:id}: ${3:Int} @query\n\t$0\n}", true, "声明请求入参模型", "input 定义接口的输入参数"},
	{"wrap", "wrap ${1:Name}<T> {\n\tcode: Int!\n\tmsg: String!\n\tdata: T\n}", true, "声明统一响应包装器", "wrap 定义全局或分组的响应包装层"},
	{"group", "group ${1:/path} {\n\t$0\n}", true, "声明接口路由分组", "group 包含一组具有相同前缀或中间件的 HTTP 接口"},
	{"scalar", "scalar ${1:Name}", false, "声明自定义标量", "scalar 扩展自定义基本类型"},
	{"union", "union ${1:Name}(${2:field}) {\n\t${3:key}: ${4:Type}\n\tdefault: Any\n}", true, "声明联合类型", "union 用于定义多态返回值"},
	{"enum", "enum ${1:Name} {\n\t$0\n}", true, "声明枚举类型", "enum 定义枚举常量"},
	{"GET", "GET /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP GET 接口", "声明一个 GET 请求路由。未显式标注的平铺参数默认提取自 URL 查询参数 (Query)"},
	{"POST", "POST /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP POST 接口", "声明一个 POST 请求路由。平铺参数默认提取自请求体 (Body) 并自动合成 XxxArgs 包装模型；若期望使用 URL 查询参数请显式标注 @query"},
	{"PUT", "PUT /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP PUT 接口", "声明一个 PUT 请求路由。平铺参数默认提取自请求体 (Body) 并自动合成 XxxArgs 包装模型；若期望使用 URL 查询参数请显式标注 @query"},
	{"DELETE", "DELETE /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP DELETE 接口", "声明一个 DELETE 请求路由。平铺参数默认提取自请求体 (Body) 并自动合成 XxxArgs 包装模型；若期望使用 URL 查询参数请显式标注 @query"},
	{"PATCH", "PATCH /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP PATCH 接口", "声明一个 PATCH 请求路由。平铺参数默认提取自请求体 (Body) 并自动合成 XxxArgs 包装模型；若期望使用 URL 查询参数请显式标注 @query"},
	{"get", "GET /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP GET 接口", "声明一个 GET 请求路由。未显式标注的平铺参数默认提取自 URL 查询参数 (Query)"},
	{"post", "POST /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP POST 接口", "声明一个 POST 请求路由。平铺参数默认提取自请求体 (Body) 并自动合成 XxxArgs 包装模型；若期望使用 URL 查询参数请显式标注 @query"},
	{"put", "PUT /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP PUT 接口", "声明一个 PUT 请求路由。平铺参数默认提取自请求体 (Body) 并自动合成 XxxArgs 包装模型；若期望使用 URL 查询参数请显式标注 @query"},
	{"delete", "DELETE /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP DELETE 接口", "声明一个 DELETE 请求路由。平铺参数默认提取自请求体 (Body) 并自动合成 XxxArgs 包装模型；若期望使用 URL 查询参数请显式标注 @query"},
	{"patch", "PATCH /${1:path} => ${2:func}(${3:input}): ${4:Output}", true, "HTTP PATCH 接口", "声明一个 PATCH 请求路由。平铺参数默认提取自请求体 (Body) 并自动合成 XxxArgs 包装模型；若期望使用 URL 查询参数请显式标注 @query"},
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

	// 0. 判断是否正在输入元属性（在 [ ... ] 中）
	bracketIdx := strings.LastIndex(prefix, "[")
	closeBracketIdx := strings.LastIndex(prefix, "]")
	if bracketIdx != -1 && bracketIdx > closeBracketIdx {
		afterBracket := strings.TrimSpace(prefix[bracketIdx+1:])
		snipFormat := protocol.InsertTextFormatSnippet
		propKind := protocol.CompletionItemKindProperty
		enumKind := protocol.CompletionItemKindEnumMember

		// 如果在 seal= 后面
		if strings.HasSuffix(afterBracket, "seal=") || strings.HasSuffix(afterBracket, "seal =") {
			symbols := buildSymbolTable(filename)
			extractFallbackSymbols(content, filename, symbols)
			classKind := protocol.CompletionItemKindClass
			for _, sym := range symbols {
				if sym.Kind == "Model" {
					name := sym.Name
					detail := fmt.Sprintf("%s (信封模型)", name)
					doc := fmt.Sprintf("定义于: %s:%d", filepath.Base(sym.Filename), sym.Line)
					items = append(items, protocol.CompletionItem{
						Label:         name,
						Kind:          &classKind,
						Detail:        &detail,
						Documentation: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc},
						InsertText:    &name,
					})
				}
			}
			return items
		}

		// 如果在 ctype= 后面
		if strings.HasSuffix(afterBracket, "ctype=") || strings.HasSuffix(afterBracket, "ctype =") {
			ctypes := []struct{ val, doc string }{
				{"stream", "二进制原始数据流下载 (如文件导出)"},
				{"zip", "ZIP 压缩流下载"},
				{"multipart", "multipart/form-data 文件上传"},
				{"json", "application/json 默认格式"},
				{"form", "application/x-www-form-urlencoded 表单"},
			}
			for _, ct := range ctypes {
				v := ct.val
				d := ct.doc
				items = append(items, protocol.CompletionItem{
					Label:      v,
					Kind:       &enumKind,
					Detail:     &d,
					InsertText: &v,
				})
			}
			return items
		}

		// 在中括号内部提供可用元属性补全
		metaEntries := []struct {
			label, insertText, detail, doc string
		}{
			{"seal", "seal=${1:SealModel}", "seal=WrapModel - 声明加解封信封", "在 RequestMeta 或 ResponseMeta 中声明该接口在物理传输层使用的加解封信封。"},
			{"ctype", "ctype=${1|stream,zip,multipart,json,form|}", "ctype=... - 指定传输格式", "指定请求或响应传输层的 Content-Type 编码格式。"},
			{"ws", "ws=true", "ws=true - 升级为 WebSocket 端点", "将该接口声明为双工 WebSocket 全双工长连接端点。"},
			{"prefix", "prefix=\"${1:/api}\"", "prefix=\"...\" - 路由前缀", "为路由分组或端点指定自定义路径前缀。"},
		}
		for _, m := range metaEntries {
			label := m.label
			it := m.insertText
			dt := m.detail
			doc := m.doc
			items = append(items, protocol.CompletionItem{
				Label:            label,
				Kind:             &propKind,
				Detail:           &dt,
				Documentation:    protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc},
				InsertText:       &it,
				InsertTextFormat: &snipFormat,
			})
		}
		return items
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
	extractFallbackSymbols(content, filename, symbols)
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
