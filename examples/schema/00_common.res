# 1. 通用 JSON 响应包装器
wrap ResData<T> {
    code: Int!
    msg: String!
    data: T
}

# 2. 自定义标量：网络传输层使用 Int（int64）
scalar IntTime: Int
