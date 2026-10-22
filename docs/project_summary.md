# Resgen 项目阶段性总结 (1.0-RC)

**`resgen` 不仅仅是一个提效工具，它更是一套内嵌了行业最佳实践的 API 设计标准与生命周期规范。** 在最近的开发迭代中，我们对 Resgen 的核心架构、DSL 语法灵活性以及 API 文档系统进行了深度的标准化与增强。

## 🛠️ 核心架构升级

### 1. 统一响应渲染契约
- **错误路径标准化**：强制所有生成的错误响应在包装器中传递 `nil` 数据 (`e.r.Bind{Wrapper}(ctx, nil, err)`)。
- **类型安全保障**：通过 `GoType` 与 `JSONName` 的分离，确保生成的 Go 结构体既符合 Golang 语法（如处理 `any` 接口指针问题），又能在 JSON/文档中保持 DSL## 2. 最近更新 (2026-05-08)

### 2.1 API 文档全自动化集成 (Zero-Config Integration)
- **零代码集成**：在 `resgen.yaml` 中开启 `enable_api_docs: true` 后，生成的 `Engine` 会在绑定注册器时自动挂载 `/docs` (HTML 预览) 和 `/docs/json` (API 定义) 路由。
- **内置渲染引擎**：文档注册逻辑现在直接集成在 `engine.gen.go` 中，利用 `ServerContext` 接口实现框架无关的 HTML 渲染。
- **资源内嵌**：利用 `go:embed` 技术，将 `api.html` 和 `api.json` 直接编译进二进制，实现单文件部署。
- **UI 视觉升级**：恢复了基于 Tailwind CSS 的高级暗黑主题文档系统，并移除了外链依赖（通过本地生成），确保在内网环境下也能完美显示。

### 2.2 架构解耦与重构
- **移除外部依赖包**：删除了项目根目录下的 `pkg` 文件夹。现在所有运行时的辅助逻辑（如引擎、包装器等）均通过生成器在目标项目中实例化，符合“生成即用”的设计哲学。
- **生成器代码重构**：优化了 `internal/generator` 逻辑，将文档生成抽离为独立模块，提升了生成效率和代码可维护性。
- **泛型约束优化**：为 `Engine` 增加了 `Context[T]` 约束，确保了引擎内部调用渲染方法的类型安全性。
- **自动归一化**：HTTP 方法支持小写定义（如 `get`），生成器会自动归一化为标准大写。
- **安全约束**：针对 `GET` 方法实施了“禁止嵌套结构体”的编译检查，强制 API 参数扁平化，提升 RESTful 规范性。

## 📄 API 文档系统进化

### 1. 深度泛型支持
- 交互式文档现在可以完美解析并还原 `ResData<User>` 这种复杂的泛型包装结构。
- 通过集成 `returnTypeDSL` 字段，解决了浏览器渲染带尖括号类型的 HTML 转义问题。

### 2. 交互体验优化
- **自动化代码示例**：文档中的 `curl` 和 `fetch` 示例现在会自动解析 Query 参数并转化为带参数的 URL。
- **解包模式支持**：对于解包后的参数，文档现在能正确识别其真正的源（Query/Path/Body），消除了之前的硬编码显示错误。
- **状态码动态展示**：支持 `@status` 指令，文档实时反映非 200 的成功状态码。
- **权限标识一键提取与高亮显示**：支持通过 `resgen.yaml` 的约定映射，将鉴权装饰器中的权限码自动提取到 `api.json` 的 `permission` 字段。同时在 `api.html` 导航目录及详情页的头部均能展现醒目的锁图标与金色高对比度 Badge，极佳地解决了前端对于接口权限识别的耦合痛点。

## 🛠️ 最新版本特性演进 (Latest Updates)

### 1. 响应包装器体系与树形结构支持 (TreeRes & Wrappers)
- **`TreeRes<T>` 树形包装器**：内置对树形层级数据结构（`items: [T!]!`, `total: Int!`）的原生支持。
- **自引用模型保护**：完善了递归自引用树节点（如 `CategoryTreeNode` 包含 `children: [CategoryTreeNode!]`）的支持，并在 API 交互式文档中加入循环引用智能保护，标记为 `Self-ref Tree Node`，彻底防止前端页面渲染无限递归卡死。

### 2. 文件与流式响应强类型载体 (LocalFileDownload & RenderStream)
- **`LocalFileDownload` 强类型载体**：封装 `FilePath`（物理路径）、`Filename`（下载文件名）、`ContentType`（MIME 类型），作为业务 Resolver 返回文件时的统一载体。
- **`RenderStream` 强类型签名**：`ServerContextBase` 契约升级为 `RenderStream(code int, localFileDownload LocalFileDownload)`，彻底消除 `any` 反射与协议侵入。
- **流式接口纯净输出**：文件下载接口成功响应默认以裸流传输（`wrap=none`），失败时优雅退化为 JSON 错误响应。

### 3. 多协议 Tags 按需精准推导机制 (Precision Tag Generation)
- **全端点使用场景精准分析**：所有模型（包括业务 Entity、Input 结构体以及 `ResData` / `ListRes` / `TreeRes` / `PageData` 等包装器）的字段 Tag，严格根据它们在所有被引用的端点中实际使用的协议类型（`ctype` / `etype`）全集生成。
- **零 Tag 冗余污染**：只在普通 JSON 接口中使用的模型，绝不生成多余的 `xml`、`yaml` 等无用 Tag；在 XML 接口中被使用的模型自动获得对应大小写风格的 `xml:"..."` 标签。

### 4. 多层结构 Form 表单提交支持 (Nested Form Input)
- **深层嵌套与点语法绑定**：`[ctype=form]` 支持复杂嵌套子模型，框架适配层与测试套件支持 `address.city=Shenzhen` 等点分层级键名与递归绑定。

### 6. Seal 自动加包与解包机制 (Seal Wrapping & Unwrapping) & OpenAPI 3.0.3 规范化升级
- **传输层与业务层解耦**：在网络传输物理数据与后端 Handler 入参/出参之间提供自动化加包（Wrap）与解包（Unwrap/Peel）逻辑，后端业务代码专注于干净的业务 Model，零协议/密文外壳污染；
- **外壳模型通用化**：任意 `input` 或 `type` 模型均可作为加包/解包的外壳结构（通过 `[seal=...]` 声明）；
- **`@payload` 载体标注增强**：支持在外壳模型及普通 API 模型中独立使用 `@payload(ModelName)` 标注密文字段解包后的明文契约，API 交互式文档以卡片形式优雅呈现；
- **OpenAPI 3.0.3 极简标准化导出**：全面消除 JSON 根层级非标冗余键（`modules`、`models`、`enums`），所有框架拓展元数据统一前缀为 `x-resgen-` 和 `x-res-`，完全兼容第三方 OpenAPI 校验器与前端渲染工具；
- **真实 Mock 交互与代码示例**：API 文档智能解析参数与 Schema，自动生成符合格式的精美 `curl` 与 `JavaScript/Fetch` 可运行示例。

### 7. 请求参数绑定统一与自动包装合成 (Request Argument Binding Consistency)
- **非 GET 方法平铺参数严格统一**：消除 `POST`、`PUT`、`PATCH`、`DELETE` 方法下单个标量与多参数的绑定歧义；所有非 GET 接口中未标注的平铺参数统一反序列化自请求体 (Body)；
- **自动合成包装结构体**：针对平铺入参，编译器自动生成强类型的 `XxxArgs` 输入模型，并在 OpenAPI 文档与接口注册逻辑中同步生成 `requestBody` 契约；
- **显式定向提取支持**：在非 GET 接口中，若确实需要从 URL 查询参数或路径提取，仅需显式声明 `@query` 或 `@path`，语义清晰严谨。

### 8. 原生 LSP 语言服务器与编辑器生态增强 (Native LSP & Tooling Enhancements)
- **HTTP 动词与核心指令 Hover 实时提示**：重构 `internal/lsp` 词法定位机制，支持在 `GET`、`POST`、`PUT`、`DELETE`、`PATCH` 及 `@query`、`@alias` 等位置悬停时展示详尽的 Markdown 规范与参数提取规则指导；
- **上下文感知智能补全**：更新 `builtinDirectives` 和 `builtinKeywords` 补全说明，在编码阶段即时向开发者同步各方法的参数绑定规则与模版引导；
- **完备的协议级自动化测试**：补充针对 LSP 词法提取、定义定位、补全推荐与实时 Hover 的全套端到端单元测试，为 VSCode 及 JetBrains 系列 IDE 提供稳定可靠的原生服务。

## 📂 项目文档清单


我们已在 `docs/` 目录下整理了以下指南：
- [DSL 语法指南](file:///d:/项目/resgen/docs/dsl_guide.md)：涵盖模型、包装器、参数绑定、多协议与标量等。
- [响应处理机制](file:///d:/项目/resgen/docs/response_handling.md)：详细说明了错误映射、状态码与包装器契约。
- [项目演进总结](file:///d:/项目/resgen/docs/project_summary.md)：了解 Resgen 的最新特性与架构演进。
