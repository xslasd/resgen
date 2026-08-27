# 示例 5：状态码与错误包装器控制
# 展示：
# - 接口级 state 覆盖默认成功状态码
# - 接口级 wrap 覆盖组级或全局的错误包装器
# - 不同场景下的 wrap=none（禁用包装）
# - 组级 wrap 与接口级 wrap 的优先级规则
module StatusDemo

# 分页专用包装器
wrap PageData<T> {
    list: T!
    total: Int!
    page: Int!
}

type Product {
    id: Int!
    name: String!
    price: Float!
}

input CreateProductInput {
    name: String!
    price: Float!
}

# 默认遵循配置文件的 default_wrap: 成功 200，错误用 ResData 包装
group /products {
    # 默认成功 200 + ResData 包装
    GET /:id => GetProduct(id: Int @path): Product
    # 创建资源：成功状态码改为 201 Created
    POST /create => CreateProduct(input: CreateProductInput): Product [state=201]
    # 批量更新：成功状态码改为 202 Accepted（异步处理）
    POST /batch-update => BatchUpdate(ids: [Int!]!): String [state=202]
    # 无内容删除：成功状态码改为 204 No Content，直接返回空响应（wrap=none）
    DELETE /:id => DeleteProduct(id: Int @path): String [state=204, wrap=none]
    # 接口级覆盖 wrap：使用分页包装器替换默认的 ResData
    GET /list => ListProducts(page: Int, size: Int): PageData<[Product!]!> [wrap=none]
}

# 无全局包装的组：每个接口自定义
group /raw {
    # 直接返回原始对象，不包装
    GET /product/:id => GetRawProduct(id: Int @path): Product [wrap=none]
    # 需要包装时在接口级单独声明
    GET /products => GetRawProducts(page: Int): ResData<[Product]> [wrap=ResData]
}
