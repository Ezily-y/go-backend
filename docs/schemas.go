package docs

import "github.com/getkin/kin-openapi/openapi3"

// ---- schema 构造小工具 ----
//
// kin-openapi v0.149 的 SchemaRef 只有 Ref + Value 两个导出字段
//（旧版的 .Schema 已移除），类型用 &openapi3.Types{"..."} 表示。
// 以下 helper 让各 schema 定义保持紧凑。

// ref 指向 components/schemas 里的定义，供各处复用。
func ref(name string) *openapi3.SchemaRef {
	return openapi3.NewSchemaRef("#/components/schemas/"+name, nil)
}

// s 返回一个带类型的裸 schema（无引用）。
func s(types ...string) *openapi3.Schema {
	return &openapi3.Schema{Type: (*openapi3.Types)(&types)}
}

// str / int / num / bool / arr 是常用类型的简写。
func str() *openapi3.Schema     { return s("string") }
func integer() *openapi3.Schema { return s("integer") }
func number() *openapi3.Schema  { return s("number") }
func boolean() *openapi3.Schema { return s("boolean") }
func obj() *openapi3.Schema     { return s("object") }

// arr 返回数组 schema，items 为元素的 $ref 名。
func arr(item string) *openapi3.Schema {
	a := s("array")
	a.Items = ref(item)
	return a
}

// prop 把 schema 包成 SchemaRef，供 Properties 使用。
func prop(sc *openapi3.Schema) *openapi3.SchemaRef {
	return &openapi3.SchemaRef{Value: sc}
}

// desc 为 schema 附加描述并返回自身，便于链式书写。
func desc(sc *openapi3.Schema, d string) *openapi3.Schema {
	sc.Description = d
	return sc
}

// ex 为 schema 附加示例并返回自身。
func ex(sc *openapi3.Schema, v any) *openapi3.Schema {
	sc.Example = v
	return sc
}

// enum 为 schema 附加枚举并返回自身。
func enum(sc *openapi3.Schema, values ...any) *openapi3.Schema {
	sc.Enum = values
	return sc
}

// nullable 在 3.1 下用 type 数组 ["string","null"] 表达可空。
func nullable(sc *openapi3.Schema) *openapi3.Schema {
	if sc.Type == nil {
		sc.Type = &openapi3.Types{"null"}
		return sc
	}
	appended := append(sc.Type.Slice(), "null")
	sc.Type = (*openapi3.Types)(&appended)
	return sc
}

// ---- components.schemas ----

// schemas 返回 OpenAPI components.schemas。
//
// 与 internal/model、internal/model/dto 的真实字段保持一一对应；
// 改实体字段时同步更新这里，否则文档会与实现漂移（编译器无法察觉）。
func schemas() openapi3.Schemas {
	return openapi3.Schemas{
		// ---- 通用信封 ----
		"Error": prop(withRequired(withProps(desc(obj(), "统一错误响应体；code 非 0 表示失败，5xx 对外只返回通用文案。"), map[string]*openapi3.SchemaRef{
			"code":    prop(desc(integer(), "业务错误码，0 表示成功")),
			"message": prop(desc(str(), "提示信息")),
		}), "code", "message")),

		// ---- 实体 ----
		"User":   userSchema(),
		"Role":   roleSchema(),
		"APIKey": apiKeySchema(),

		// ---- 认证 ----
		"LoginRequest":          loginRequestSchema(),
		"RefreshRequest":        refreshRequestSchema(),
		"ChangePasswordRequest": changePasswordSchema(),
		"TokenPair":             tokenPairSchema(),
		"LoginResponse":         loginResponseSchema(),

		// ---- 用户 ----
		"CreateUserRequest": createUserSchema(),
		"UpdateUserRequest": updateUserSchema(),
		"UserPage":          userPageSchema(),

		// ---- API Key ----
		"CreateAPIKeyRequest":  createKeyRequestSchema(),
		"CreateAPIKeyResponse": createKeyResponseSchema(),
		"APIKeyPage":           apiKeyPageSchema(),
		"APIKeyEnableRequest":  apiKeyEnableSchema(),

		// ---- 外部工具 ----
		"ToolPingData": toolPingDataSchema(),

		// ---- 系统 ----
		"HealthData": healthDataSchema(),
		"ReadyData":  readyDataSchema(),
		"SystemInfo": systemInfoSchema(),
	}
}

// registry 返回 schema 名 -> *Schema 的映射，供 refSchema 解析引用本体。
//
// 与 schemas() 保持同源：schemas() 构造 SchemaRef，registry() 取其 Value。
func registry() map[string]*openapi3.Schema {
	all := schemas()
	out := make(map[string]*openapi3.Schema, len(all))
	for k, v := range all {
		if v != nil && v.Value != nil {
			out[k] = v.Value
		}
	}
	return out
}

// securitySchemes 定义两套互不相干的鉴权：
//
//   - BearerAuth：JWT，走 Authorization 头，覆盖常规业务接口；
//   - ApiKeyAuth：API Key，走 X-API-Key 头，只覆盖 /api/v1/tool/* 外部工具区。
//
// 两套与 internal/middleware 的 JWT / APIKey 中间件一一对应，改动中间件时同步这里。
func securitySchemes() openapi3.SecuritySchemes {
	return openapi3.SecuritySchemes{
		"BearerAuth": &openapi3.SecuritySchemeRef{
			Value: &openapi3.SecurityScheme{
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
				Description:  "登录 POST /auth/login 获取 access_token，请求头填 `Bearer <token>`。",
			},
		},
		"ApiKeyAuth": &openapi3.SecuritySchemeRef{
			Value: &openapi3.SecurityScheme{
				Type:        "apiKey",
				In:          "header",
				Name:        "X-API-Key",
				Description: "外部工具用密钥，由 POST /api/v1/apikeys 创建；也可用 `Authorization: ApiKey <key>`。",
			},
		},
	}
}

// roleSchema 对应 model.Role。
func roleSchema() *openapi3.SchemaRef {
	return prop(enum(desc(str(), "角色：admin=超级管理员，editor=编辑者，viewer=访客"),
		"admin", "editor", "viewer"))
}

// userSchema 对应 model.User。PasswordHash 的 json tag 为 "-"，不出现在响应中。
func userSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(desc(obj(), "用户资料。密码哈希永不返回（json:\"-\"）。"), map[string]*openapi3.SchemaRef{
		"id":         prop(desc(integer(), "用户 ID")),
		"username":   prop(ex(str(), "admin")),
		"nickname":   prop(ex(str(), "超级管理员")),
		"role":       roleSchema(),
		"email":      prop(ex(withFormat(str(), "email"), "admin@example.com")),
		"status":     prop(ex(desc(integer(), "1=启用 0=禁用"), 1)),
		"created_at": prop(withFormat(str(), "date-time")),
		"updated_at": prop(withFormat(str(), "date-time")),
	}), "id", "username", "role", "status"))
}

// apiKeySchema 对应 model.APIKey。KeyHash 的 json tag 为 "-"，永不返回。
func apiKeySchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(desc(obj(), "API Key 元信息（不含明文；明文只在创建时返回一次）。"), map[string]*openapi3.SchemaRef{
		"id":         prop(desc(integer(), "密钥 ID")),
		"name":       prop(ex(str(), "my-tool")),
		"key_prefix": prop(ex(str(), "gk_ab12")),
		"user_id":    prop(desc(integer(), "归属用户 ID")),
		"role":       roleSchema(),
		"enabled":    prop(desc(boolean(), "是否启用")),
		"last_used":  prop(nullable(withFormat(str(), "date-time"))),
		"created_at": prop(withFormat(str(), "date-time")),
		"expired_at": prop(desc(nullable(withFormat(str(), "date-time")), "null 表示永不过期")),
	}), "id", "name", "key_prefix", "enabled"))
}

func loginRequestSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"username": prop(ex(minLen(maxLen(str(), 64), 3), "admin")),
		"password": prop(ex(minLen(maxLen(withFormat(str(), "password"), 128), 6), "admin123456")),
	}), "username", "password"))
}

func refreshRequestSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"refresh_token": prop(ex(str(), "eyJhbGciOi...")),
	}), "refresh_token"))
}

func changePasswordSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"old_password": prop(minLen(withFormat(str(), "password"), 6)),
		"new_password": prop(minLen(maxLen(withFormat(str(), "password"), 128), 6)),
	}), "old_password", "new_password"))
}

func tokenPairSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"access_token":  prop(ex(desc(str(), "访问令牌（HS256）"), "eyJhbGciOi...")),
		"refresh_token": prop(ex(str(), "eyJhbGciOi...")),
		"expires_in":    prop(ex(desc(integer(), "访问令牌有效期（秒）"), 7200)),
		"token_type":    prop(ex(str(), "Bearer")),
	}), "access_token", "refresh_token", "expires_in", "token_type"))
}

// loginResponseSchema 对应 dto.LoginResponse：令牌对内嵌 + user。
func loginResponseSchema() *openapi3.SchemaRef {
	tp := tokenPairSchema().Value
	return prop(withRequired(withProps(desc(obj(), "登录成功：令牌对 + 用户资料。"), map[string]*openapi3.SchemaRef{
		"access_token":  prop(tp.Properties["access_token"].Value),
		"refresh_token": prop(tp.Properties["refresh_token"].Value),
		"expires_in":    prop(tp.Properties["expires_in"].Value),
		"token_type":    prop(tp.Properties["token_type"].Value),
		"user":          ref("User"),
	}), "access_token", "refresh_token", "expires_in", "user"))
}

func createUserSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"username": prop(minLen(maxLen(str(), 64), 3)),
		"password": prop(minLen(withFormat(str(), "password"), 6)),
		"nickname": prop(maxLen(str(), 64)),
		"role":     roleSchema(),
		"email":    prop(withFormat(str(), "email")),
	}), "username", "password", "role"))
}

// updateUserSchema：所有字段可选，空值表示不修改（对应 dto.UpdateUserRequest 的指针字段）。
func updateUserSchema() *openapi3.SchemaRef {
	return prop(withProps(desc(obj(), "部分更新：未提供的字段保持不变。"), map[string]*openapi3.SchemaRef{
		"nickname": prop(str()),
		"role":     roleSchema(),
		"email":    prop(withFormat(str(), "email")),
		"status":   prop(enum(integer(), 0, 1)),
	}))
}

func createKeyRequestSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"name": prop(minLen(maxLen(str(), 64), 2)),
		"role": roleSchema(),
		"days": prop(ex(desc(minMax(integer(), 1, 3650), "有效天数，0 表示永不过期"), 90)),
	}), "name", "role"))
}

func createKeyResponseSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(desc(obj(), "key 只在此响应中出现一次，之后无法再次获取明文。"), map[string]*openapi3.SchemaRef{
		"id":         prop(desc(integer(), "密钥 ID")),
		"name":       prop(str()),
		"key":        prop(ex(desc(str(), "明文密钥（仅此一次）"), "gk_ab12cd34...")),
		"key_prefix": prop(ex(str(), "gk_ab12")),
		"expired_at": prop(nullable(withFormat(str(), "date-time"))),
	}), "id", "name", "key", "key_prefix"))
}

// userPageSchema 对应 response.Page 的 data 形状：{list,total,page,page_size}。
func userPageSchema() *openapi3.SchemaRef {
	return prop(pageBody("User"))
}

func apiKeyPageSchema() *openapi3.SchemaRef {
	return prop(pageBody("APIKey"))
}

// pageBody 构造分页 data：list 为指定实体数组。
func pageBody(entity string) *openapi3.Schema {
	return withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"list":      prop(arr(entity)),
		"total":     prop(desc(integer(), "总条数")),
		"page":      prop(desc(integer(), "当前页码")),
		"page_size": prop(desc(integer(), "每页条数")),
	}), "list", "total", "page", "page_size")
}

// apiKeyEnableSchema 对应 APIKeyHandler.Update 的内联请求体 {enabled: bool}。
func apiKeyEnableSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"enabled": prop(desc(boolean(), "true=启用 false=禁用")),
	}), "enabled"))
}

// toolPingDataSchema 对应 /api/v1/tool/ping 的 data 字段。
// pong 恒为 true；role 是该 API Key 的角色，用于验证鉴权链路已打通。
func toolPingDataSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(desc(obj(), "连通性探测结果"), map[string]*openapi3.SchemaRef{
		"pong": prop(desc(boolean(), "恒为 true")),
		"role": roleSchema(),
	}), "pong", "role"))
}

// healthDataSchema 对应 SystemHandler.Health 的 data 字段。
func healthDataSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"status":   prop(enum(desc(str(), "up=健康，degraded=数据库不可用"), "up", "degraded")),
		"database": prop(enum(str(), "up", "down")),
		"uptime":   prop(ex(str(), "1h30m0s")),
		"version":  prop(ex(str(), "dev")),
	}), "status", "database", "uptime", "version"))
}

// readyDataSchema 对应 SystemHandler.Ready 的 data 字段。
func readyDataSchema() *openapi3.SchemaRef {
	return prop(withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"status":  prop(ex(str(), "ok")),
		"uptime":  prop(str()),
		"version": prop(str()),
	}), "status", "uptime", "version"))
}

// systemInfoSchema 对应 SystemHandler.Info 的 data 字段。
func systemInfoSchema() *openapi3.SchemaRef {
	mem := withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"alloc_mb":       prop(number()),
		"total_alloc_mb": prop(number()),
		"sys_mb":         prop(number()),
		"num_gc":         prop(integer()),
	}), "alloc_mb", "total_alloc_mb", "sys_mb", "num_gc")

	body := withProps(obj(), map[string]*openapi3.SchemaRef{
		"version":    prop(str()),
		"go_version": prop(ex(str(), "go1.25.1")),
		"goroutines": prop(integer()),
		"uptime":     prop(str()),
		"mem":        prop(mem),
	})
	return prop(withRequired(body, "version", "go_version", "goroutines", "uptime", "mem"))
}

// ---- schema 字段级修饰（返回 *Schema 以便链式调用） ----

func withFormat(sc *openapi3.Schema, f string) *openapi3.Schema {
	sc.Format = f
	return sc
}

func minLen(sc *openapi3.Schema, n uint64) *openapi3.Schema {
	sc.MinLength = n
	return sc
}

func maxLen(sc *openapi3.Schema, n uint64) *openapi3.Schema {
	sc.MaxLength = &n
	return sc
}

func minMax(sc *openapi3.Schema, min, max float64) *openapi3.Schema {
	sc.Min, sc.Max = &min, &max
	return sc
}
