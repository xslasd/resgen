# Resgen for JetBrains IDEs 🚀

为 JetBrains 系列商业版 IDE（GoLand, IntelliJ IDEA Ultimate, WebStorm 等）提供官方 Resgen DSL 支持，不仅包含基础的语法高亮，还深度集成了 **原生 LSP (Language Server Protocol) 服务**。

## 🌟 核心特性
1. **实时语法检查（Diagnostics）**：写错语法时实时显示红波浪线报错。
2. **代码格式化（Formatting）**：支持快捷键自动格式化 `.res` 文件。
3. **跳转到定义（Go to Definition）**：支持按住 Ctrl / Cmd 点击类型名称进行跳转。
4. **悬浮提示（Hover）**：鼠标悬停显示内置指令和类型说明。
5. **语法高亮与代码颜色**：保留了经典的橙色关键字、洋红色键名等优化配色。

> **⚠️ 注意事项**：LSP 高级特性依赖 JetBrains 官方的 `LspServerSupportProvider` 接口，该接口目前**仅在商业付费版 IDE**（如 GoLand, IDEA Ultimate）中受支持，社区版（Community Edition）暂不支持原生 LSP 接入，将回退至基本的 TextMate 语法高亮模式。

## 🛠️ 构建与编译指南

插件已重构为标准的 IntelliJ Platform Gradle 工程，如果需要自行编译打包，请执行以下操作：

1. 在此目录（`editors/jetbrains`）下打开终端或使用 IDE 导入该目录。
2. 运行 Gradle 构建命令：
   - Windows 终端：`.\gradlew.bat buildPlugin`
   - Mac / Linux 终端：`./gradlew buildPlugin`
   - 使用 IDE：展开右侧 Gradle 面板，双击运行 `Tasks -> intellij -> buildPlugin`
3. 稍等片刻，编译完成后，会在 `build/distributions/` 目录下生成打包好的插件压缩包（例如 `resgen-jetbrains-1.0.0.zip`）。

## 📦 安装与配置指南

1. **环境依赖准备**：插件提供的高级特性依赖您的本地环境。请确保您的电脑已经正确安装了 `resgen` 二进制文件，并确保它存在于系统的环境变量 `PATH` 中。
2. **打开 IDE 设置**：`File` -> `Settings` (Windows) 或 `IDE Name` -> `Settings` (macOS)。
3. **进入插件管理**：点击左侧的 `Plugins`。
4. **从磁盘安装**：点击顶部中间的 **⚙️ (齿轮图标)**，选择 **`Install Plugin from Disk...`**。
5. **选择插件包**：浏览并选择上一步生成的 `build/distributions/resgen-jetbrains-1.0.0.zip` 压缩包。
6. **重启 IDE**：安装完成后，按提示重启 IDE 即可生效。

## 关于文件扩展名冲突

由于 `.res` 扩展名在某些环境中可能与 ReScript 等插件冲突，本插件额外支持了 **`.resgen`** 扩展名。

- 如果您遇到冲突提示，可以选择 **Resgen DSL Support** 作为默认处理插件。
- 也可以将您的设计文件后缀名修改为 `.resgen`，以获得更稳定的开发体验。

---
更多信息请参考 [Resgen 主项目仓库](../../README.md)。
