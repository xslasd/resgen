# Resgen for JetBrains IDEs 🚀

为 JetBrains 系列 IDE（GoLand, IntelliJ IDEA, WebStorm 等）提供官方 Resgen DSL 支持。

## 安装指南（推荐方法）

为了简化部署并避免手动配置，我们已经为您准备好了标准插件包：

1. **打开 IDE 设置**：`File` -> `Settings` (Windows) 或 `IDE Name` -> `Settings` (macOS)。
2. **进入插件管理**：点击左侧的 `Plugins`。
3. **从磁盘安装**：点击顶部中间的 **⚙️ (齿轮图标)**，选择 **`Install Plugin from Disk...`**。
4. **选择目录**：浏览并选择本目录下的 **`resgen-plugin`** 文件夹。
5. **重启 IDE**：安装完成后，重启 IDE 即可生效。

## 关于文件扩展名冲突

由于 `.res` 扩展名在某些环境中可能与 ReScript 等插件冲突（如图所示），本插件已额外支持了 **`.resgen`** 扩展名。

- 如果您遇到冲突提示，可以选择 **Resgen DSL Support** 作为默认处理插件。
- 也可以将您的设计文件后缀名修改为 `.resgen`，以获得更稳定的开发体验。

## 代码颜色

本插件预设了符合 Resgen 规范的颜色：
- **关键字 (橙色)**: `type`, `input`, `module` 等。
- **字段键 (洋红色)**: 冒号前的字段名称。
- **类型 (白色)**: 冒号后的类型定义。

---
更多信息请参考 [Resgen 主项目仓库](../../README.md)。

---
更多信息请参考 [Resgen 主项目仓库](../../README.md)。
