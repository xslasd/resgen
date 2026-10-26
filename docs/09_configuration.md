# 配置全景规范 (resgen.yaml)

Resgen 的代码生成逻辑、命名规则、协议映射与自动化文档系统均可通过项目根目录下的 `resgen.yaml` 配置文件进行深度定制。

---

## 1. 生产级配置全景模板

```yaml
generator:
  # 1. 代码生成基本配置
  package: "resolver"           # 生成的 Go 包名 (默认: resolver)
  default_case: "snake"         # 默认字段命名风格: snake (推荐), camel, lower, keep
  default_content_type: "json"  # 默认请求与响应的 Content-Type 别名
  default_ok_status: 200        # 接口未显式指定 state 时的默认成功 HTTP 状态码
  default_wrap: "ResData"       # 接口未显式指定 wrap 时的全局默认包装器

  # 2. 首字母缩略词白名单 (保持专业大驼峰)
  go_initialisms:
    - "ID"
    - "URL"
    - "API"
    - "HTTP"
    - "IP"
    - "UUID"
    - "SKU"
    - "TRD"

  # 3. 标量生成风格: isolation (DTO 隔离流, 默认) | direct (标量直通流)
  scalar_style: "isolation"

  # 4. API 契约与内嵌交互式文档配置
  enable_api_docs: true         # 是否开启契约与文档自动生成并内嵌部署
  base_url: "https://api.example.com" # 文档中展示的默认服务器基准路径

  # 5. 权限码映射约定
  auth_decorator: "auth"        # 代表权限的装饰器名称 (忽略大小写)
  auth_param_name: "role"       # 装饰器中代表权限码的形参字段名

  # 6. 多协议传输规格表
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

    toml:
      mime: "application/toml"
      tag: "toml"
      case: "snake"
      raw_type: "toml.Primitive"
      import_pkg: "github.com/BurntSushi/toml"
      unmarshal_fn: "toml.Unmarshal"

    text:
      mime: "text/plain"

    html:
      mime: "text/html; charset=utf-8"

# 7. 自定义标量 Go 实现路径映射
scalars:
  IntTime:
    model: "github.com/your-org/project/pkg/scalars.IntTime"
```

---

## 2. 核心配置节详细说明

### 2.1 结构体命名风格 (`default_case`)
定义生成 Go 结构体 Tag 时，字段名在传输层对应的命名风格：
- `snake`（默认）：下划线风格，如 `user_id`、`created_at`（RESTful 推荐）；
- `camel`：小驼峰风格，如 `userId`、`createdAt`；
- `lower`：纯小写无分隔，如 `userid`；
- `keep`：与 DSL 中书写的属性名完全保持一致，不改变大小写。

### 2.2 首字母缩略词 (`go_initialisms`)
在遵循 Go 官方命名规范时，缩略词应当全大写（例如 `UserID` 而非 `UserId`，`APIURL` 而非 `ApiUrl`）。

通过在 `go_initialisms` 数组中配置业务专属词汇（如 `SKU`, `SPU`, `TRD`），生成器会将包含该词汇的字段自动转为大写大驼峰：
- 输入 `out_trd_no` ➡️ 输出字段名 `OutTRDNo`
- 输入 `sku_id` ➡️ 输出字段名 `SKUID`

### 2.3 自定义标量风格 (`scalar_style`)
- **`isolation` (默认)**：生成 DTO 结构体与 `ToDTO/FromDTO` 转换逻辑。业务模型使用原生 Go 类型（如 `time.Time`），转换逻辑支持传入 `ctx` 上下文；
- **`direct`**：不生成 DTO，业务模型直接使用标量别名类型，代码极致精简。

### 2.4 文档内嵌与零代码部署 (`enable_api_docs`)
设为 `true` 后，代码生成器会执行以下操作：
1. 自动生成符合 OpenAPI 3.0.3 标准的 `docs/api.json`；
2. 自动生成基于暗黑主题的高级交互式预览页面 `docs/api.html`；
3. 通过 Go 官方 `//go:embed` 技术将静态文件直接编译进二进制；
4. 引擎在执行 `BindRegister` 时自动挂载 `GET /docs`（页面）与 `GET /docs/json`（数据），实现随服务启动零成本自动交付 API 文档。

### 2.5 权限映射约定 (`auth_decorator` & `auth_param_name`)
- 解析器在扫描端点时，会自动检索匹配该名称的装饰器（如 `@auth("admin")`）；
- 将其参数值抽取并记录在 OpenAPI 元数据的 `permission` 字段中；
- 文档界面自动为有权限保护的接口渲染金黄色挂锁 Badge 与图标。
