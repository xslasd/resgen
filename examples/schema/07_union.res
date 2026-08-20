module UnionDemo

type UnionArticle {
    title: String!
    content: String!
}

type UnionVideo {
    title: String!
    url: String!
    duration: Int!
}

enum UnionKind: String {
    article: "article"
    video: "video"
}

union ContentPayload(UnionKind) {
    article: UnionArticle
    video: UnionVideo
}

input CreatePostInput {
    # 1. 判别器字段（这里它的类型是 UnionKind，符合 ContentPayload 的要求）
    type: UnionKind!
    # 2. 联合类型字段，通过 (type) 显式关联同级的判别器字段
    payload: ContentPayload(type)
}


type ContentPostItem {
    id: Int!
    type: UnionKind!
    payload: ContentPayload(type)
}


group /posts {
    # 帖子查询接口：演示联合类型作为返回值
    GET /:id => GetPost(id: Int @path): ContentPostItem
    # 创建帖子接口 (UnionKind)
    POST / => CreatePost(input: CreatePostInput!): ContentPostItem
 }
