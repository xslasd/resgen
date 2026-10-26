# 多协议传输与 Content-Type (Protocols & Formats)

现代分布式系统往往需要在多种传输编码协议之间自如切换：对外提供标准的 JSON API，对旧系统对接 Form 表单，对外贸/金融机构提供 XML 协议，对配置服务提供 YAML/TOML。

Resgen 内置了**全场景多协议传输体系**，支持零样板代码的多协议平滑映射与按需精准 Tag 生成。

---

## 1. 协议规格全元数据模型 (ContentTypeSpec)

在 `resgen.yaml` 中，可以通过 `content_types` 字典对每一种传输协议进行声明与定制：

```yaml
generator:
  default_content_type: "json"
  default_case: "snake"
  content_types:
    json:
      mime: "application/json"
      tag: "json"
      case: "snake"
      raw_type: "json.RawMessage"
      import_pkg: "encoding/json"
      unmarshal_fn: "json.Unmarshal"

    form:
      mime: "application/x-www-form-urlencoded"
      tag: "form"
      case: "snake"

    multipart:
      mime: "multipart/form-data"
      tag: "form"
      case: "snake"

    xml:
      mime: "application/xml"
      tag: "xml"
      case: "camel"
      raw_type: "runtime.XMLRawNode"
      import_pkg: "github.com/xslasd/resgen/runtime"
      unmarshal_fn: "xml.Unmarshal"

    yaml:
      mime: "application/x-yaml"
      tag: "yaml"
      case: "snake"
      raw_type: "yaml.Node"
      import_pkg: "gopkg.in/yaml.v3"
      unmarshal_fn: "yaml.Unmarshal"
```

---

## 2. 多协议 Tags 按需精准推导机制 (Precision Tag Generation)

在传统的代码生成工具中，一个结构体往往会被无脑贴上一长串 Tag（例如同时包含 `json:"..." form:"..." xml:"..." yaml:"..." toml:"..."`），不仅让结构体极其臃肿丑陋，还会显著增加二进制体积并拖慢运行期反射性能。

Resgen 引入了**编译器全图场景精准分析机制**：
- **按需生成**：生成器会全量扫描当前项目所有模型在端点中的实际引用链路；
- **零冗余污染**：只在普通 JSON 端点中引用的模型，**绝对不会生成任何 `xml`、`yaml` 等无用 Tag**；
- **自适应风格**：在 XML 接口中引用的模型，会自动按照 XML 协议配置的命名风格生成 `xml:"PascalCase"` 或 `xml:"camelCase"` 标签。

```go
// 仅在 JSON 接口中使用的模型：仅生成干净的 json 标签
type UserProfile struct {
    UserID   int    `json:"user_id"`
    NickName string `json:"nick_name"`
}

// 在 XML 接口中被引用的模型：按 camel 命名风格自动注入 xml 标签
type InvoiceItem struct {
    ItemID   int     `json:"item_id" xml:"itemId"`
    Amount   float64 `json:"amount" xml:"amount"`
}
```

---

## 3. 多层结构 Form 表单提交支持 (Nested Form Input)

传统基于 `application/x-www-form-urlencoded` 的表单提交很难表达嵌套对象。Resgen 完整支持基于点分语法（Dot Notation）的多层嵌套模型自动反序列化。

### 3.1 DSL 声明
```graphql
input AddressInput {
    city:   String!
    street: String!
}

input ComplexFormInput {
    name:    String!
    age:     Int!
    address: AddressInput! # 嵌套子模型
}

group /forms {
    POST /submit [ctype=form] => SubmitForm(input: ComplexFormInput!): String
}
```

### 3.2 客户端 HTTP 提交载荷
客户端直接提交平铺的标准表单参数，子属性通过点号 `.` 连接：
```http
POST /forms/submit HTTP/1.1
Content-Type: application/x-www-form-urlencoded

name=Alice&age=28&address.city=Shenzhen&address.street=HighTechPark
```

Resgen 框架的执行器会自动递归解构成结构化入参 `input.Address.City`，对业务代码完全透明！

---

## 4. 接口级双向协议切换 (Input & Output Protocols)

在 DSL 中，通过以下元属性精准控制端点的编解码协议：

- **请求协议 (`ctype`)**：定义请求体由哪种格式反序列化；
- **成功响应协议 (`ctype`)**：定义成功返回时调用哪个渲染器；
- **错误响应协议 (`etype`)**：定义抛出异常时退化为哪种渲染格式。

```graphql
# 1. 提交 XML，成功返回 JSON
POST /pay/notify [ctype=xml] => PayNotify(input: XmlNoticeInput!): NoticeResult [ctype=json]

# 2. 提交 YAML 配置，返回纯文本
POST /config/apply [ctype=yaml] => ApplyConfig(input: YamlConfigInput!): String [ctype=text]
```
