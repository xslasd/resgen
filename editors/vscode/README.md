# Resgen DSL Support for VS Code 🚀

为 Resgen API 设计 DSL（`.res` 文件）提供官方支持。

## 什么是 Resgen？

`resgen` 不仅仅是一个工具，它是一套经过深思熟虑的 **API 设计标准规范**。它定义了一套极简、强类型且极具表现力的 **DSL (Domain Specific Language) 语法**，旨在解决 RESTful API 开发中契约不一致、文档滞后及类型安全缺失的痛点。

本插件为 Resgen 的设计理念提供在编辑器中的无缝体验。

## 主要特性

- ✨ **深度语法高亮**：对关键字（`type`, `input`, `module`, `group` 等）提供精准色彩区分。
- 🌐 **HTTP 语义支持**：高亮显示标准 HTTP 方法（`GET`, `POST`, `PUT`, `DELETE` 等）。
- 🛡️ **修饰符与验证器**：清晰标记装饰器（`@auth`）与校验规则（`@minLen`）。
- 📝 **注释与字符串**：完善的基础语法支持，让 DSL 编写更丝滑。
- 🔥 **智能代码提示 (IntelliSense & Snippets)**：支持 `module`、`type`、`input`、`group`、各 HTTP 方法及 `@query` 等常用标签的一键 Tab 补全展开，大幅提高 API 设计效率！
- 🧹 **代码格式化 (Formatting)**：支持通过 `Shift + Alt + F` 一键对齐美化代码风格，强制统一缩进与括号布局（基于 AST 重构，保留并规范注释）。
- 🔗 **跳转到定义 (Go to Definition)**：支持按住 `Ctrl` / `Cmd` 点击引用类型（如 Model、Scalar 或 Decorator 标签），光标能自动精准跳转到定义行，支持同一工作区下跨文件的关联跳转。

## 前置准备与配置

为了使用格式化与跳转定义等高级 LSP 功能，需要你的系统已安装 `resgen` 命令行工具。

1. **编译 resgen**（在 `resgen` 根目录下执行）：
   - **Windows 平台**：
     ```bash
     go build -o resgen.exe main.go
     ```
   - **macOS / Linux 平台**：
     ```bash
     go build -o resgen main.go
     ```
2. **配置程序路径**：
   - **推荐**：将编译出的可执行文件所在的目录添加至系统的环境变量 `PATH` 中（如 Windows 系统上的环境变量 `Path`）。
   - **备选**：如果不想修改环境变量，也可以在 VS Code 的全局 `settings.json`（或项目级的 `.vscode/settings.json`）中直接配置可执行文件的绝对路径：
     - **Windows 示例**（注意路径中的斜杠转义）：
       ```json
       "resgen.path": "D:\\项目\\resgen\\resgen.exe"
       ```
     - **macOS / Linux 示例**：
       ```json
       "resgen.path": "/absolute/path/to/resgen"
       ```

## 安装与开发指南

1. **打包插件**：
   在 `editors/vscode` 目录下依次运行以下命令安装依赖并编译生成 extension 包：
   ```bash
   npm install
   npm run compile
   ```
2. **手动加载插件**：
   将 `editors/vscode` 整个文件夹复制（或在 Windows 下通过软链接链接）到您的 VS Code 插件目录下。为了符合 VS Code 的插件命名规范（格式为 `<publisher>.<name>-<version>` 或 `<publisher>.<name>`），建议将文件夹重命名为 **`resgen.resgen-vscode`**：
   - **Windows**：
     `%USERPROFILE%\.vscode\extensions\resgen.resgen-vscode`
   - **macOS / Linux**：
     `~/.vscode/extensions/resgen.resgen-vscode`
3. 重启 VS Code 即可享受极致的 API 设计体验。

---
了解更多关于 Resgen 的信息，请访问 [Resgen 主项目仓库](../../README.md)。
