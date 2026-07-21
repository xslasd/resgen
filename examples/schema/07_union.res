type UnionArticle {
    title: String!
    content: String!
}

type UnionVideo {
    title: String!
    url: String!
    duration: Int!
}

scalar UnionKind: String

# 定义一个联合类型，预期判别器字段的类型必须为 String
union ContentPayload(UnionKind) {
    "article": UnionArticle
    "video": UnionVideo
    default: Any
}

input CreatePostInput {
    # 1. 判别器字段（这里它的类型是 UnionKind，符合 ContentPayload 的要求）
    type: UnionKind!
    
    # 2. 联合类型字段，通过 (type) 显式关联同级的判别器字段
    payload: ContentPayload(type)
}

group /posts {
    # 创建帖子接口
    POST / => CreatePost(input: CreatePostInput!): Any
}
