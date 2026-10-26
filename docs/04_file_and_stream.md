# 文件流与上传下载实战 (File & Stream)

在 Web 开发中，文件上传与流式下载往往涉及底层的 `multipart` 分片解析、二进制流缓冲、HTTP 响应头控制以及失败时的降级处理。

Resgen 提供了行业级的一体化文件处理方案，将入参映射为 `*multipart.FileHeader`，将出参映射为强类型的 `*LocalFileDownload` 载体，兼顾卓越的类型安全与开箱即用的开发体验。

---

## 1. 文件上传场景 (Input)

### 1.1 单文件与多文件上传 DSL 定义
通过声明 `ctype=multipart`，并在输入模型中声明 `File` 类型的属性：

```graphql
module FileService

# 单文件上传入参
input UploadAvatarInput {
    avatar:      File!     # 必填单文件
    description: String    # 附带的表单普通字段
}

# 批量文件上传入参
input BatchUploadInput {
    documents:   [File!]!  # 必填非空文件切片
    category:    String!
}

group /files {
    POST /avatar [ctype=multipart] => UploadAvatar(input: UploadAvatarInput!): UploadResult
    POST /batch  [ctype=multipart] => BatchUpload(input: BatchUploadInput!): BatchResult
}
```

### 1.2 生成的 Go 模型代码
在生成的 Go 结构体中，`File` 会自动映射为标准库的 `multipart.FileHeader`：

```go
type UploadAvatarInput struct {
    Avatar      multipart.FileHeader `form:"avatar"`
    Description *string              `form:"description"`
}

type BatchUploadInput struct {
    Documents   []multipart.FileHeader `form:"documents"`
    Category    string                 `form:"category"`
}
```

### 1.3 文件约束与验证 (FileRule)
支持在 `File` 字段上挂载校验器，在进入业务逻辑前完成文件大小（字节数）和 MIME 类型的白名单自检：

```graphql
# 声明文件校验器：限制最大字节数与允许的 MIME 格式列表
validator fileRule(maxSize: Int, types: [String!], msg: String)

input UploadAvatarInput {
    avatar: File! @fileRule(
        maxSize = 2097152, # 2MB
        types = ["image/jpeg", "image/png", "image/webp"],
        msg = "头像仅支持 JPG/PNG/WebP 格式且不能超过 2MB"
    )
}
```

---

## 2. 文件流式下载场景 (Output) 与 `LocalFileDownload`

### 2.1 强类型载体结构详解
为了彻底消除传统方案中返回 `any`、`[]byte` 或裸写 `http.ResponseWriter` 带来的反射与协议侵入，Resgen 定义了统一的强类型文件下载载体：

```go
// LocalFileDownload 专门用于封装本地物理文件下载信息
type LocalFileDownload struct {
    FilePath    string // 本地物理磁盘文件路径
    Filename    string // 客户端下载保存时的文件名（自动设置 Content-Disposition 响应头）
    ContentType string // 文件 MIME 类型。若 DSL 中已声明已知格式（如 ctype=pdf），留空即可自动注入契约 MIME
}
```

### 2.2 典型下载场景实战

#### 场景 A：静态已知格式文件导出（如 PDF、CSV、Excel、ZIP）
当接口返回固定已知格式时，推荐在出参上声明对应的文件别名（如 `ctype=pdf`），并设置 `wrap=none`（避免文件流被 JSON 外壳污染）：

```graphql
group /export {
    # 导出报表 PDF：成功时以 application/pdf 裸流下载，失败时降级为 JSON 错误
    GET /report/:id => ExportReport(id: Int! @path): File [ctype=pdf, etype=json, wrap=none]

    # 导出数据 CSV
    GET /csv => ExportCSV(): File [ctype=csv, etype=json, wrap=none]
}
```

**业务层实现代码**：
```go
func (h *FileHandler) ExportReport(ctx context.Context, id int) (*resolver.LocalFileDownload, error) {
    // 业务只需指定物理路径与客户端下载保存的文件名，MIME 由框架根据 ctype=pdf 强行注入
    return &resolver.LocalFileDownload{
        FilePath: "./storage/reports/annual_2026.pdf",
        Filename: "2026年度财务分析报表.pdf",
    }, nil
}
```

#### 场景 B：多格式动态通用附件下载
如果接口需要根据不同的文件 ID 下载不同类型的文件（如可能是图片、视频或压缩包），可以使用通用二进制流 `ctype=stream`，并在业务层动态指定 `ContentType`：

```graphql
GET /download/:file_id => DownloadAttachment(file_id: String! @path): File [ctype=stream, etype=json, wrap=none]
```

**业务层实现代码**：
```go
func (h *FileHandler) DownloadAttachment(ctx context.Context, fileID string) (*resolver.LocalFileDownload, error) {
    fileMeta := getFileMeta(fileID)
    return &resolver.LocalFileDownload{
        FilePath:    fileMeta.LocalPath,
        Filename:    fileMeta.OriginalName,
        ContentType: fileMeta.MimeType, // 动态指定具体 MIME，如 "video/mp4"
    }, nil
}
```

---

## 3. 内置高频流式文件格式别名速查表

在 DSL 中使用 `ctype=...` 时，可直接使用以下内置别名，框架会自动映射为标准的 HTTP `Content-Type` 响应头：

| 别名 (Alias) | 对应标准 MIME 类型 | 适用场景说明 |
| :--- | :--- | :--- |
| **`stream`** | `application/octet-stream` | 二进制原始数据流（通用兜底格式） |
| **`pdf`** | `application/pdf` | PDF 电子文档下载 |
| **`zip`** | `application/zip` | ZIP 压缩包流式下载 |
| **`csv`** | `text/csv; charset=utf-8` | CSV 表格数据导出 |
| **`xlsx`** | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | 现代 Excel 工作簿导出 |
| **`xls`** | `application/vnd.ms-excel` | 早期二进制 Excel 文件导出 |
| **`docx`** | `application/vnd.openxmlformats-officedocument.wordprocessingml.document` | Word 办公文档导出 |
| **`png`** | `image/png` | PNG 无损图像资源输出 |
| **`jpeg`** / **`jpg`** | `image/jpeg` | JPEG 摄影照片输出 |

---

## 4. 弹性设计：成功裸流传输与失败优雅降级

通过在 DSL 中组合 `[wrap=none, etype=json]`：
- **执行成功时**：框架调用底层 `RenderStream` 输出纯粹的文件二进制流与下载头，零 JSON 外壳干扰；
- **业务出错时**（如文件不存在、权限不足抛出 `error`）：框架会自动**安全退化**，利用统一的包装器输出标准的 JSON 结构化错误响应，而不是将错误信息直接写入损坏的文件流中！
