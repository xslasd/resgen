module EnvAuth

# ---------- 信封声明（直接使用 input 与 type 声明物理信封结构） ----------
input CipherWrap<T> {
    body: String! @payload(T)              # 载体字段：body 里装 T 的明文（默认 json）
    ver:  Int     @header                  # 明文格式版本（密文外层）
    kid:  String  @header                  # 密钥版本
}

input Sm4Wrap<T> {
    body: String! @payload(T, msgpack)     # 指定 msgpack 编码契约
    kid:  String  @header
}

type CipherWrapOut<T> {                    # 出参加壳信封
    cipher: String! @payload(T)
}

type GzWrap<T> {                           # 透明信封：内容可读
    data: String! @payload(T)
    enc:  String
}

# ---------- 业务模型 ----------
input EnvLoginReq  { login_id: String!  password: String! }
input EnvPayReq    { order_id: String!  amount: Int! }
input EnvToken     { access_token: String!  expires_in: Int! }
type  EnvPayResult { ok: Boolean! }

input RawPayloadReq {
    raw_data: String! @payload(EnvLoginReq) # 普通非信封接口字段，使用 @payload 辅助说明内部明文结构
}

# ---------- 接口编排 ----------
group /env_auth {
    # ① 单载荷 + 双向
    POST /login [seal=CipherWrap<EnvLoginReq>] => EnvLogin(input: EnvLoginReq): EnvToken [seal=CipherWrapOut<EnvToken>]

    # ② 裸名（载荷由 input 自动推导）
    POST /login/bare [seal=CipherWrap] => EnvLoginBare(input: EnvLoginReq): EnvToken

    # ③ 混合参数：header 明文 + body 密文
    POST /login/v2 [seal=CipherWrap<EnvLoginReq>] => EnvLoginV2(
        device_id: String! @header,
        input:     EnvLoginReq
    ): EnvToken

    # ④ 另一 Codec 实例
    POST /login/v3 [seal=Sm4Wrap<EnvLoginReq>] => EnvLoginV3(input: EnvLoginReq): EnvToken

    # ⑤ 出参加壳（入参明文）
    POST /legacy/login => EnvLegacyLogin(input: EnvLoginReq): EnvToken [seal=CipherWrapOut]

    # ⑥ 无 seal 场景下直接在普通字段上使用 @payload 标注内部结构
    POST /raw_payload => SubmitRawPayload(input: RawPayloadReq): EnvPayResult
}
