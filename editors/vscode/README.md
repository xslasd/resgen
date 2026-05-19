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

## 安装指南

1. 将 `editors/vscode` 文件夹复制到您的 VS Code 插件目录：
   - **Windows**: `%USERPROFILE%\.vscode\extensions`
   - **macOS/Linux**: `~/.vscode/extensions`
2. 重启 VS Code 即可享受极致的 API 设计体验。

---
了解更多关于 Resgen 的信息，请访问 [Resgen 主项目仓库](../../README.md)。
