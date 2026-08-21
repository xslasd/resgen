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

# ----------------------------------------------------
# 多级结构 + 联合类型：嵌套对象与数组切片
# ----------------------------------------------------

# 多级嵌套模型：单个帖子项（包含作者与联合类型 payload）
input PostDetailInput {
    author: String!
    type: UnionKind!
    payload: ContentPayload(type)
}

# 多级复合入参模型：同时包含普通字段、嵌套单对象、嵌套数组切片中的多态
input BatchCreatePostInput {
    batchName: String!
    # 1. 嵌套单对象中的联合类型 (可空)
    pinnedPost: PostDetailInput
    # 2. 嵌套数组切片中的联合类型 (列表)
    items: [PostDetailInput!]!
}

# 批量创建返回结果模型
type BatchCreatePostResult {
    batchName: String!
    total: Int!
    items: [ContentPostItem!]!
}

group /posts {
    # 帖子查询接口：演示联合类型作为返回值
    GET /:id => GetPost(id: Int @path): ContentPostItem
    # 单帖创建接口：基础联合类型 (UnionKind)
    POST / => CreatePost(input: CreatePostInput!): ContentPostItem
    # 批量发布接口：多级结构 (嵌套对象 + 切片列表) 中的联合类型入参
    POST /batch => BatchCreatePost(input: BatchCreatePostInput!): BatchCreatePostResult
}
