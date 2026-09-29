package docs

import (
	"fmt"
	"net/http"
)

// ScalarCDN 是 Scalar API Reference 的 standalone 构建（jsDelivr）。
//
// 选它而非自托管的理由：服务端只返回一页 HTML，零打包、零 node 依赖，
// 仓库里不进任何前端产物。代价是依赖第三方 CDN —— 离线环境请改用
// 自托管：把 @scalar/api-reference 的 dist 拷到 web/scalar/ 再改这里的 URL。
const ScalarCDN = "https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.72.1/dist/browser/standalone.js"

// UI 返回 Scalar 文档页的 HTML。
//
// 页面把 spec 以 JSON 内联进 <script>，再交给 Scalar 渲染 ——
// 这样不额外发一次 /openapi.json 请求，避免 CORS/相对路径问题。
func UI() string {
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
  html, body { margin: 0; padding: 0; height: 100%%; }
</style>
</head>
<body>
<div id="app"></div>
<script id="openapi-spec" type="application/json">%s</script>
<script src="%s"></script>
<script>
  (function () {
    var spec = document.getElementById('openapi-spec').textContent;
    if (typeof Scalar === 'undefined') {
      // CDN 不可达时给出可诊断的提示，而不是白屏
      document.getElementById('app').innerHTML =
        '<pre style="padding:24px;font:14px/1.6 monospace">' +
        'Scalar CDN 加载失败（可能无外网）。\n' +
        '可访问 /openapi.json 获取 OpenAPI 3.1 文档。</pre>';
      return;
    }
    Scalar.createApiReference('#app', {
      spec: { content: JSON.parse(spec) },
      theme: 'default',
      hideDarkModeToggle: false,
      metaData: { title: %q }
    });
  })();
</script>
</body>
</html>
`, Title, JSON(), ScalarCDN, Title)
}

// UIHandler 返回渲染文档页的处理器。
func UIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(UI()))
	}
}
