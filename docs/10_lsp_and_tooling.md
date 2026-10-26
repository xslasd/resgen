# 原生 LSP 与开发工具生态 (LSP & Tooling)

为了提供丝滑的 IDE 开发体验，Resgen 内置了原生的 **Language Server Protocol (LSP)** 语言服务器，并官方发布了 **VSCode** 与 **JetBrains 系列 IDE (GoLand, IntelliJ IDEA 等)** 插件，带来第一梯队的开发支持。

---

## 1. 原生 LSP 架构与启动命令

Resgen LSP 服务直接内置在 `resgen` 主二进制中，通过标准输入输出（Stdio）与各类现代 IDE 进行 JSON-RPC 通信，零外部依赖、秒级冷启动。

### 1.1 命令行启动
```bash
resgen lsp
```
当 IDE 插件激活后，会自动在后台调用此命令启动服务。

---

## 2. LSP 核心特性全景

### 2.1 实时悬停指导 (Hover Hints)
- **HTTP 动词绑定规则智能指导**：
  将鼠标悬停在 `GET`、`POST`、`PUT`、`DELETE`、`PATCH` 上时，LSP 会即时展示其参数绑定机制（例如向开发者解释：*“非 GET 请求中未标注的平铺参数默认全量从 Body 反序列化并合成 Args 结构体，需从 URL Query 提取时需显式追加 @query”*）；
- **内置指令文档直达**：悬停在 `@query`、`@alias`、`@path`、`@header`、`@customBind`、`@payload` 以及 `[seal=...]` 等元属性上时，提供规范释义、应用场景与代码示例。

### 2.2 实时语义诊断与语法纠错 (Diagnostics)
在编码过程中，LSP 会即时进行全量 AST 语法与高阶语义检查：
- 检查自定义标量是否非法继承了 `File` 类型；
- 检查 `union` 联合类型的判别器是否绑定了合法的 `enum`；
- 检查 `File` 类型的合法边界（如禁止在出参声明文件数组 `[File]`，防止非法的结构体嵌套文件流）；
- 校验匿名 Body 载荷在参数列表中的唯一性；
- 检查 `[seal=...]` 引用的信封模型是否已经在工作区中定义。

### 2.3 符号秒级跳转 (Go-to-Definition)
在任何接口入参模型名、出参模型名、泛型参数或引用字段上按下 `F12` 或按住 `Ctrl + 点击`，LSP 会瞬间精准定位并跳转至对应的 `type`、`input`、`enum`、`union` 源码定义行。

### 2.4 上下文感知智能补全 (Completions)
- **指令补全**：键入 `@` 自动联想 `@query`、`@path`、`@header`、`@alias`、`@payload`、`@customBind`、`@customValidate`；
- **类型补全**：在属性或接口返回值冒号 `:` 后，自动提示内置标量以及当前项目中所有已定义的业务模型；
- **方括号元属性补全**：在 `[...]` 内部输入时，自动提示 `seal=WrapModel`、`ctype=stream/json/form`、`wrap=ResData`、`state=200` 等代码片段（Snippets）；
- **路由模版 Snippets**：键入 `GET`、`POST`、`group` 回车，自动展开规范的路由结构模版。

### 2.5 代码自动格式化 (Document Formatting)
支持标准的编辑器格式化快捷键（如 `Shift + Alt + F`），一键自动对齐字段、缩进指令、排版路由箭头。

---

## 3. IDE 插件安装与配置指南

### 3.1 VSCode 插件接入
1. **安装**：
   - 源码位于仓库 [`editors/vscode`](../editors/vscode) 目录；
   - 支持通过 VSIX 离线包安装：在 VSCode 扩展面板选择 `Install from VSIX...` 并选中打包文件。
2. **配置**：
   默认自动寻找系统 `PATH` 中的 `resgen` 可执行文件；若需指定绝对路径，可在 `settings.json` 中配置：
   ```json
   {
       "resgen.lsp.path": "C:/Users/your-name/go/bin/resgen.exe"
   }
   ```

### 3.2 JetBrains 系列 IDE 接入 (GoLand / IDEA)
1. **安装**：
   - 源码位于仓库 [`editors/jetbrains`](../editors/jetbrains) 目录，已适配 IntelliJ Platform 原生 LSP 扩展体系；
   - 支持编译打包为 `.zip` 插件包后通过 `Plugins -> Install Plugin from Disk...` 快速安装。
2. **享受原生高亮与语义分析**：
   安装后即刻拥有 `.res` 专属文件图标、TextMate 语法高亮、实时错误红线提示与方法参数提示。
