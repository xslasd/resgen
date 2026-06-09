# 示例 6：文件上传与下载
# 展示：
# - 单文件上传（@fileRule 限制大小与格式）
# - 多文件字段的 multipart 表单
# - 文件下载（流式响应）
# - 错误时安全退化为 JSON
module FileDemo

# 文件校验器：大小（字节）、允许的 MIME 类型、自定义错误消息
validator fileRule(maxSize: Int!, types: [String!]!, msg: String)

# 单文件上传的输入模型
input UploadAvatarInput {
    userId: Int!
    # 仅允许 JPG/PNG，最大 2MB（2097152 字节）
    avatar: File! @fileRule(maxSize: 2097152, types: ["image/jpeg", "image/png"], msg: "头像仅支持 JPG/PNG 格式，且不超过 2MB")
}

# 混合表单：同时包含普通字段与文件
input UploadDocumentInput {
    title: String!
    description: String
    # 仅允许 PDF，最大 10MB
    document: File! @fileRule(maxSize: 10485760, types: ["application/pdf"], msg: "仅支持 PDF 格式，且不超过 10MB")
    # 可选封面图片
    cover: File @fileRule(maxSize: 1048576, types: ["image/jpeg", "image/png", "image/webp"], msg: "封面图仅支持 JPG/PNG/WebP 格式，且不超过 1MB")
}

# 上传结果模型
type UploadResult {
    fileUrl: String!
    fileSize: Int!
    mimeType: String!
}

group /files [wrap=ResData] {
    # 单文件上传：头像
    POST /avatar[ctype=multipart] => UploadAvatar(input: UploadAvatarInput): ResData<UploadResult> [state=201]
    # 混合表单上传：文档 + 封面
    POST /document[ctype=multipart] => UploadDocument(input: UploadDocumentInput): ResData<UploadResult> [state=201]
    # 文件下载：成功响应为字节流，失败响应为 JSON（使用 ResData 包装错误）
    GET /download/:id => DownloadFile(id: Int @path): String [ctype=text, etype=json, wrap=ResData]
    # 导出 CSV：同为流式文本响应，失败退化 JSON
    GET /export/csv => ExportCsv(ids: String @query): String [ctype=text, etype=json, wrap=ResData]
}
