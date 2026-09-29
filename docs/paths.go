package docs

import (
	"strconv"

	"github.com/getkin/kin-openapi/openapi3"
)

// paths 返回 OpenAPI paths：16 个操作，与 internal/router/router.go 注册的路由一一对应。
//
// 路由变更时（新增/删除/改路径）需同步更新本文件；
// docs_test.go 会校验 spec 与实际路由表一致，防止漂移。
//
// kin-openapi 没有 Operation 的链式 setter（WithRequestBody/WithParameters 不存在），
// 因此这里用命令式写法：先构造 Operation，再逐字段赋值。
func paths() *openapi3.Paths {
	p := openapi3.NewPaths()

	// ---- 系统 ----
	p.Set("/health", &openapi3.PathItem{Get: op("健康检查", "系统")})
	setStatus(p.Find("/health").Get, 200, jsonOK("服务与数据库状态", "HealthData"))
	setStatus(p.Find("/health").Get, 503, jsonOK("数据库不可用（status=degraded）", "HealthData"))

	p.Set("/ready", &openapi3.PathItem{Get: op("存活探针", "系统")})
	setStatus(p.Find("/ready").Get, 200, jsonOK("进程存活", "ReadyData"))

	// ---- 认证 ----
	p.Set("/api/v1/auth/login", &openapi3.PathItem{Post: op("用户登录", "认证")})
	login := p.Find("/api/v1/auth/login").Post
	login.RequestBody = bodyReq("登录信息", "LoginRequest")
	setStatus(login, 200, jsonOK("登录成功，返回令牌与用户", "LoginResponse"))
	setStatus(login, 401, errResp("账号或密码错误"))
	setStatus(login, 429, errResp("触发限流（1 秒内最多 5 次）"))

	p.Set("/api/v1/auth/refresh", &openapi3.PathItem{Post: op("刷新令牌", "认证")})
	refresh := p.Find("/api/v1/auth/refresh").Post
	refresh.RequestBody = bodyReq("刷新令牌", "RefreshRequest")
	setStatus(refresh, 200, jsonOK("刷新成功", "TokenPair"))
	setStatus(refresh, 401, errResp("刷新令牌无效或已过期"))
	setStatus(refresh, 429, errResp("触发限流（1 秒内最多 10 次）"))

	p.Set("/api/v1/auth/me", &openapi3.PathItem{Get: withBearer(op("获取当前登录用户信息", "认证"))})
	me := p.Find("/api/v1/auth/me").Get
	setStatus(me, 200, jsonOK("当前用户资料", "User"))
	setStatus(me, 401, errResp("未登录或令牌无效"))

	p.Set("/api/v1/auth/password", &openapi3.PathItem{Put: withBearer(op("修改密码", "认证"))})
	pwd := p.Find("/api/v1/auth/password").Put
	pwd.RequestBody = bodyReq("密码信息", "ChangePasswordRequest")
	setStatus(pwd, 200, jsonOKEmpty("修改成功"))
	setStatus(pwd, 400, errResp("原密码错误或新密码不合法"))
	setStatus(pwd, 401, errResp("未登录或令牌无效"))

	// ---- 系统信息 ----
	p.Set("/api/v1/system/info", &openapi3.PathItem{Get: withBearer(op("运行时信息", "系统"))})
	info := p.Find("/api/v1/system/info").Get
	setStatus(info, 200, jsonOK("版本、Go 运行时与内存概况", "SystemInfo"))
	setStatus(info, 401, errResp("未登录或令牌无效"))

	// ---- 用户管理（仅 admin） ----
	p.Set("/api/v1/users", &openapi3.PathItem{})
	users := p.Find("/api/v1/users")
	users.Get = withBearer(op("用户列表", "用户管理"))
	users.Get.Parameters = append(pageParams(),
		queryStr("keyword", "模糊匹配用户名/昵称", 64),
		queryEnum("role", "角色过滤", "admin", "editor", "viewer"),
	)
	setStatus(users.Get, 200, jsonOK("分页用户列表", "UserPage"))
	setStatus(users.Get, 401, errResp("未登录或令牌无效"))
	setStatus(users.Get, 403, errResp("非 admin 无权访问"))

	users.Post = withBearer(op("创建用户", "用户管理"))
	users.Post.RequestBody = bodyReq("用户信息", "CreateUserRequest")
	setStatus(users.Post, 200, jsonOK("创建成功", "User"))
	setStatus(users.Post, 401, errResp("未登录或令牌无效"))
	setStatus(users.Post, 403, errResp("非 admin 无权访问"))
	setStatus(users.Post, 409, errResp("用户名已存在"))

	p.Set("/api/v1/users/{id}", &openapi3.PathItem{})
	user := p.Find("/api/v1/users/{id}")
	user.Get = withBearer(op("获取用户详情", "用户管理"))
	user.Get.Parameters = openapi3.Parameters{pathID("用户 ID")}
	setStatus(user.Get, 200, jsonOK("用户详情", "User"))
	setStatus(user.Get, 404, errResp("用户不存在"))

	user.Put = withBearer(op("更新用户", "用户管理"))
	user.Put.Parameters = openapi3.Parameters{pathID("用户 ID")}
	user.Put.RequestBody = bodyReq("待更新字段（空值不修改）", "UpdateUserRequest")
	setStatus(user.Put, 200, jsonOK("更新后的用户", "User"))
	setStatus(user.Put, 404, errResp("用户不存在"))

	user.Delete = withBearer(op("删除用户", "用户管理"))
	user.Delete.Parameters = openapi3.Parameters{pathID("用户 ID")}
	setStatus(user.Delete, 200, jsonOKEmpty("删除成功"))
	setStatus(user.Delete, 404, errResp("用户不存在"))

	// ---- API Key ----
	p.Set("/api/v1/apikeys", &openapi3.PathItem{})
	keys := p.Find("/api/v1/apikeys")
	keys.Get = withBearer(op("API Key 列表", "API Key"))
	keys.Get.Parameters = append(pageParams(),
		queryStr("keyword", "模糊匹配别名", 64),
	)
	setStatus(keys.Get, 200, jsonOK("分页密钥列表（不含明文）", "APIKeyPage"))
	setStatus(keys.Get, 401, errResp("未登录或令牌无效"))

	keys.Post = withBearer(op("创建 API Key", "API Key"))
	keys.Post.RequestBody = bodyReq("密钥信息", "CreateAPIKeyRequest")
	setStatus(keys.Post, 200, jsonOK("创建成功；key 明文只出现这一次", "CreateAPIKeyResponse"))
	setStatus(keys.Post, 401, errResp("未登录或令牌无效"))
	setStatus(keys.Post, 409, errResp("密钥别名已存在"))

	p.Set("/api/v1/apikeys/{id}", &openapi3.PathItem{})
	key := p.Find("/api/v1/apikeys/{id}")
	key.Patch = withBearer(op("启用/禁用 API Key", "API Key"))
	key.Patch.Parameters = openapi3.Parameters{pathID("密钥 ID")}
	key.Patch.RequestBody = bodyReq("启用状态", "APIKeyEnableRequest")
	setStatus(key.Patch, 200, jsonOKEmpty("状态已更新"))
	setStatus(key.Patch, 401, errResp("未登录或令牌无效"))
	setStatus(key.Patch, 404, errResp("密钥不存在"))

	key.Delete = withBearer(op("删除 API Key", "API Key"))
	key.Delete.Parameters = openapi3.Parameters{pathID("密钥 ID")}
	setStatus(key.Delete, 200, jsonOKEmpty("删除成功"))
	setStatus(key.Delete, 401, errResp("未登录或令牌无效"))
	setStatus(key.Delete, 404, errResp("密钥不存在"))

	// ---- 外部工具（API Key 鉴权，与 JWT 区完全隔离） ----
	p.Set("/api/v1/tool/ping", &openapi3.PathItem{})
	ping := p.Find("/api/v1/tool/ping")
	ping.Get = withApiKey(op("连通性探测", "外部工具"))
	setStatus(ping.Get, 200, jsonOK("密钥有效，返回角色", "ToolPingData"))
	setStatus(ping.Get, 401, errResp("缺少 X-API-Key 或密钥无效/已禁用"))

	return p
}

// setStatus 把一条响应挂到指定状态码（覆盖同码默认值，如 400/500）。
// Responses 是私有 map，只能通过 Set 方法写入。
func setStatus(o *openapi3.Operation, status int, r *openapi3.ResponseRef) {
	if o.Responses == nil {
		o.Responses = openapi3.NewResponses()
	}
	o.Responses.Set(strconv.Itoa(status), r)
}
