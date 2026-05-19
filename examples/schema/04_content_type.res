# 示例 4：请求与响应的 Content-Type 控制
# 展示：
#   - 请求端 ctype（json / form / multipart / 自定义）
#   - 响应端 ctype（json / text / 自定义）
#   - 错误响应端 etype
#   - 组级 ctype 默认值与接口级覆盖
module ContentTypeDemo

# 公共定义（ResData）已在 00_common.res 中声明，此处直接使用

type Report {
    title:   String!
    summary: String!
}

input JsonInput {
    title:   String!
    content: String!
}

input FormInput {
    name:  String!
    email: String!
}

# ── POST 接口的三种请求 Content-Type ─────────────────────
group /content [wrap=ResData] {
    # 默认：请求体为 JSON（可省略 ctype=json）
    POST /json => SubmitJson(input: JsonInput): ResData<String>

    # 请求体为 URL 编码表单（application/x-www-form-urlencoded）
    POST /form [ctype=form] => SubmitForm(input: FormInput): ResData<String>

    # 请求体为 multipart/form-data（文件上传场景在示例 6 详细展示）
    POST /multipart [ctype=multipart] => SubmitMultipart(title: String!): ResData<String>
}

# ── GET 接口的响应 Content-Type ──────────────────────────
group /export {
    # 响应为纯文本，错误时退化为 JSON（错误使用 ResData 包装）
    GET /text => ExportText(): String [ctype=text, etype=json, wrap=ResData]

    # 响应为 JSON（默认，可省略）
    GET /json => ExportJson(): Report [ctype=json]

    # 响应为 XML（需在 resgen.yaml content_type_aliases 中定义 xml 别名）
    GET /xml => ExportXml(): Report [ctype=xml, etype=json, wrap=ResData]
}
