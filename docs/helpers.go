package docs

import "github.com/getkin/kin-openapi/openapi3"

// ---- 响应与请求包装 ----
//
// 所有接口统一返回 {"code","message","data"}（见 internal/response）。
// code=0 成功；非 0 失败。5xx 对外只返回"服务器内部错误"。

// dataBody 把 data schema 包进统一成功响应体。
func dataBody(data *openapi3.Schema) *openapi3.Schema {
	return withRequired(withProps(obj(), map[string]*openapi3.SchemaRef{
		"code":    prop(ex(desc(integer(), "固定 0"), 0)),
		"message": prop(ex(str(), "success")),
		"data":    prop(data),
	}), "code", "message", "data")
}

// emptyBody 成功但无 data 内容（data 会被置为 {}）。
func emptyBody() *openapi3.Schema {
	return dataBody(obj())
}

// refSchema 取 components/schemas 的 schema 本体，供包装使用。
//
// 注意不能写成 ref(name).Value —— ref() 只构造 $ref 字符串，Value 恒为 nil，
// 那样拿到的会是 nil，导致 spec 里出现空的 data 字段。
func refSchema(name string) *openapi3.Schema {
	sc, ok := registry()[name]
	if !ok {
		panic("docs: 未注册的 schema 引用: " + name)
	}
	return sc
}

// errResp 构造统一错误响应（3.1 下用具体 HTTP 状态码作 key）。
func errResp(description string) *openapi3.ResponseRef {
	return &openapi3.ResponseRef{Value: &openapi3.Response{
		Description: &description,
		Content: openapi3.Content{
			"application/json": &openapi3.MediaType{Schema: ref("Error")},
		},
	}}
}

// jsonResp 构造成功 JSON 响应。
func jsonResp(desc string, body *openapi3.Schema) *openapi3.ResponseRef {
	return &openapi3.ResponseRef{Value: &openapi3.Response{
		Description: &desc,
		Content: openapi3.Content{
			"application/json": &openapi3.MediaType{
				Schema: &openapi3.SchemaRef{Value: body},
			},
		},
	}}
}

// bodyReq 构造 JSON 请求体。
func bodyReq(description, schemaName string) *openapi3.RequestBodyRef {
	return &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
		Description: description,
		Required:    true,
		Content: openapi3.Content{
			"application/json": &openapi3.MediaType{Schema: ref(schemaName)},
		},
	}}
}

// ---- 参数 ----

// pageParams 返回 page / page_size 查询参数。
//
// Default 必须是标量值（int/float/string），不能是指针 ——
// JSON Schema 校验会拒绝 *int 这类类型。
func pageParams() openapi3.Parameters {
	page := integer()
	page.Default, page.Min = 1, ptrF(1)

	size := integer()
	size.Default, size.Min, size.Max = 20, ptrF(1), ptrF(100)

	return openapi3.Parameters{
		{Value: &openapi3.Parameter{Name: "page", In: "query",
			Description: "页码，默认 1", Schema: prop(page)}},
		{Value: &openapi3.Parameter{Name: "page_size", In: "query",
			Description: "每页条数，默认 20，最大 100", Schema: prop(size)}},
	}
}

// pathID 统一的 :id 路径参数。
func pathID(description string) *openapi3.ParameterRef {
	sc := integer()
	sc.Min = ptrF(1)
	return &openapi3.ParameterRef{Value: &openapi3.Parameter{
		Name: "id", In: "path", Required: true,
		Description: description, Schema: prop(sc),
	}}
}

// queryStr 返回一个字符串查询参数。
func queryStr(name, description string, limit uint64) *openapi3.ParameterRef {
	return &openapi3.ParameterRef{Value: &openapi3.Parameter{
		Name: name, In: "query", Description: description,
		Schema: prop(maxLen(str(), limit)),
	}}
}

// queryEnum 返回一个枚举查询参数。
func queryEnum(name, description string, values ...any) *openapi3.ParameterRef {
	return &openapi3.ParameterRef{Value: &openapi3.Parameter{
		Name: name, In: "query", Description: description,
		Schema: prop(enum(str(), values...)),
	}}
}

// ---- 操作构造 ----

// op 构造一个 operation 的公共骨架（含 400/500 默认错误响应）。
func op(summary, tag string) *openapi3.Operation {
	return &openapi3.Operation{
		Summary: summary,
		Tags:    []string{tag},
		Responses: openapi3.NewResponses(
			openapi3.WithStatus(400, errResp("参数校验失败")),
			openapi3.WithStatus(500, errResp("服务器内部错误")),
		),
	}
}

// withBearer 给操作挂上 BearerAuth 安全要求（JWT，常规业务接口）。
func withBearer(o *openapi3.Operation) *openapi3.Operation {
	o.Security = &openapi3.SecurityRequirements{{"BearerAuth": nil}}
	return o
}

// withApiKey 给操作挂上 ApiKeyAuth 安全要求（X-API-Key，外部工具区）。
//
// 与 withBearer 互斥：一个 operation 只挂一种，对应 router 里不同的中间件组。
func withApiKey(o *openapi3.Operation) *openapi3.Operation {
	o.Security = &openapi3.SecurityRequirements{{"ApiKeyAuth": nil}}
	return o
}

// ok / fail 是常见响应的简写，配合 bulk 用法可把一串 setStatus 压成一次调用。
//
// 注意：Go 不允许给 openapi3.Operation 这类外部类型加方法（会报
// "cannot define new methods on non-local type"），所以这里一律用函数而非链式调用。

// jsonOK 成功响应：data 包进统一信封。
func jsonOK(desc, schemaName string) *openapi3.ResponseRef {
	return jsonResp(desc, dataBody(refSchema(schemaName)))
}

// jsonOKEmpty 成功但无业务数据（data 为 {}）。
func jsonOKEmpty(desc string) *openapi3.ResponseRef {
	return jsonResp(desc, emptyBody())
}

// ---- 小工具 ----

func ptrAny[T any](v T) *T    { return &v }
func ptrF(v float64) *float64 { return &v }

// withProps 填充 object 的 properties 并返回自身（Go 不允许给外部类型加方法，故用函数）。
func withProps(sc *openapi3.Schema, m map[string]*openapi3.SchemaRef) *openapi3.Schema {
	sc.Properties = m
	return sc
}

// withRequired 填充 object 的 required 并返回自身。
func withRequired(sc *openapi3.Schema, names ...string) *openapi3.Schema {
	sc.Required = names
	return sc
}
