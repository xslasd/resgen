# 公共声明文件：全局 wrap 和自定义标量在此统一定义，其他模块直接引用
# 注意：此文件无 module 声明，仅提供全局共享定义

# 1. 通用 JSON 响应包装器
wrap ResData<T> {
    code: Int!
    msg:  String!
    data: T
}

# 2. 自定义标量：网络传输层使用 Int（int64）
scalar IntTime: Int
