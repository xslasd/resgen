# 示例 5：状态码与错误包装器控制
# 展示：
#   - 接口级 state 覆盖默认成功状态码
#   - 接口级 wrap 覆盖组级或全局的错误包装器
#   - 不同场景下的 wrap=none（禁用包装）
#   - 组级 wrap 与接口级 wrap 的优先级规则
module StatusDemo

# 公共定义（ResData）已在 00_common.res 中声明，此处直接使用

# 分页专用包装器
wrap PageData<T> {
    list:  T!
    total: Int!
    page:  Int!
}

type Product {
    id:    Int!
    name:  String!
    price: Float!
}

input CreateProductInput {
    name:  String!
    price: Float!
}

# 组级默认：成功 200，错误用 ResData 包装
group /products [wrap=ResData] {
    # 使用组级默认：200 + ResData
    GET /:id => GetProduct(id: Int @path): ResData<Product>

    # 创建资源：成功改为 201 Created
    POST /create => CreateProduct(input: CreateProductInput): ResData<Product> [state=201]

    # 批量更新：成功改为 202 Accepted（异步处理）
    POST /batch-update => BatchUpdate(ids: [Int!]!): ResData<String> [state=202]

    # 无内容删除：成功改为 204 No Content，直接返回空响应
    DELETE /:id => DeleteProduct(id: Int @path): String [state=204, wrap=none]

    # 接口级覆盖 wrap：使用分页包装器替换 ResData
    GET /list => ListProducts(page: Int, size: Int): PageData<[Product!]!> [wrap=none]
}

# 无全局包装的组：每个接口自定义
group /raw {
    # 直接返回原始对象，不包装
    GET /product/:id => GetRawProduct(id: Int @path): Product

    # 需要包装时在接口级单独声明
    GET /products => GetRawProducts(page: Int): ResData<[Product]> [wrap=ResData]
}
