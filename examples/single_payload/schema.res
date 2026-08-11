module SinglePayloadExample

type Item {
    id: Int!
    name: String!
}

wrap Res<T> {
    code: Int!
    msg: String!
    data: T
}

group /api [wrap=Res] {
    # 接收单个数组类型的 Payload
    # JSON 请求体示例: {"items": [{"id": 1, "name": "Apple"}, {"id": 2, "name": "Banana"}]}
    POST /items => SaveItems([Item!]!): Int!
    
    # 接收单个 Any 类型的 Payload
    # JSON 请求体示例: {"data": {"anything": "you want", "number": 42}}
    POST /raw => ProcessRaw(Any): Any
}
