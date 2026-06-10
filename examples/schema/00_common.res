# 1. 通用 JSON 响应包装器
wrap ResData<T> {
    code: Int!
    msg: String!
    data: T
}

# 2. 自定义标量：网络传输层使用 Int（int64）
scalar IntTime: Int

# 3. 通用列表/分页响应包装器
wrap ListRes<T> {
    rows: [T!]! # 列表数据
    total: Int! # 总条数
}
