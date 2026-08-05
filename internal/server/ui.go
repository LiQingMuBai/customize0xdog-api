package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleOrderTestPage(w http.ResponseWriter, r *http.Request) {
	if !s.uiAuthEnabled() {
		http.Error(w, "UI_USERNAME 和 UI_PASSWORD 未配置", http.StatusServiceUnavailable)
		return
	}
	if !s.uiIsAuthed(r) {
		http.Redirect(w, r, "/ui/login", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(orderTestHTML))
}

func (s *Server) handleUILoginPage(w http.ResponseWriter, r *http.Request) {
	if !s.uiAuthEnabled() {
		http.Error(w, "UI_USERNAME 和 UI_PASSWORD 未配置", http.StatusServiceUnavailable)
		return
	}
	s.renderUILoginPage(w, "")
}

func (s *Server) handleUILoginSubmit(w http.ResponseWriter, r *http.Request) {
	if !s.uiAuthEnabled() {
		http.Error(w, "UI_USERNAME 和 UI_PASSWORD 未配置", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.renderUILoginPage(w, "表单解析失败")
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	captchaInput := strings.TrimSpace(r.FormValue("captcha"))
	captchaCode := strings.TrimSpace(r.FormValue("captcha_code"))
	captchaToken := strings.TrimSpace(r.FormValue("captcha_token"))

	if username == "" || password == "" || captchaInput == "" {
		s.renderUILoginPage(w, "请输入账号、密码、验证码")
		return
	}

	if err := s.uiVerifyCaptcha(captchaToken, captchaCode, captchaInput); err != nil {
		s.renderUILoginPage(w, "验证码错误")
		return
	}

	if username != s.cfg.UIUsername || password != s.cfg.UIPassword {
		s.renderUILoginPage(w, "账号或密码错误")
		return
	}

	s.uiSetSessionCookie(w, username)
	http.Redirect(w, r, "/ui/order", http.StatusFound)
}

func (s *Server) uiAuthEnabled() bool {
	return strings.TrimSpace(s.cfg.UIUsername) != "" && strings.TrimSpace(s.cfg.UIPassword) != ""
}

func (s *Server) uiSecret() []byte {
	return []byte(s.cfg.TeldogAPIKey + ":" + s.cfg.UIUsername + ":" + s.cfg.UIPassword)
}

func (s *Server) uiIsAuthed(r *http.Request) bool {
	c, err := r.Cookie("ui_session")
	if err != nil {
		return false
	}

	parts := strings.SplitN(strings.TrimSpace(c.Value), ".", 2)
	if len(parts) != 2 {
		return false
	}

	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	if time.Since(time.Unix(ts, 0)) > 24*time.Hour {
		return false
	}

	expect := uiHMACHex(s.uiSecret(), fmt.Sprintf("%s.%d", s.cfg.UIUsername, ts))
	return secureEqualHex(parts[1], expect)
}

func (s *Server) uiSetSessionCookie(w http.ResponseWriter, username string) {
	ts := time.Now().Unix()
	sig := uiHMACHex(s.uiSecret(), fmt.Sprintf("%s.%d", username, ts))
	val := fmt.Sprintf("%d.%s", ts, sig)

	http.SetCookie(w, &http.Cookie{
		Name:     "ui_session",
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((24 * time.Hour).Seconds()),
	})
}

func (s *Server) uiVerifyCaptcha(token string, code string, input string) error {
	parts := strings.SplitN(strings.TrimSpace(token), ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid token")
	}

	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid ts")
	}
	if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
		return fmt.Errorf("expired")
	}

	code = strings.TrimSpace(code)
	input = strings.TrimSpace(input)
	if code == "" || input == "" {
		return fmt.Errorf("empty")
	}
	if input != code {
		return fmt.Errorf("mismatch")
	}

	expect := uiHMACHex(s.uiSecret(), fmt.Sprintf("%d.%s", ts, code))
	if !secureEqualHex(parts[1], expect) {
		return fmt.Errorf("bad sig")
	}
	return nil
}

func (s *Server) renderUILoginPage(w http.ResponseWriter, errMsg string) {
	ts := time.Now().Unix()
	code := uiRandDigits(4)
	token := fmt.Sprintf("%d.%s", ts, uiHMACHex(s.uiSecret(), fmt.Sprintf("%d.%s", ts, code)))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(s.uiLoginHTML(code, token, errMsg)))
}

func (s *Server) uiLoginHTML(code string, token string, errMsg string) string {
	errMsg = strings.TrimSpace(errMsg)
	if errMsg != "" {
		errMsg = `<div class="err">` + htmlEscape(errMsg) + `</div>`
	}

	return `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>登录</title>
    <style>
      body { font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Arial; margin: 0; padding: 24px; background: #0b1020; color: #e7eaf0; }
      .wrap { max-width: 520px; margin: 0 auto; }
      h1 { margin: 0 0 16px; font-size: 20px; }
      .card { background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.10); border-radius: 12px; padding: 16px; }
      label { display: block; font-size: 12px; opacity: 0.9; margin-bottom: 6px; }
      input { width: 100%; box-sizing: border-box; background: rgba(255,255,255,0.08); border: 1px solid rgba(255,255,255,0.14); color: #e7eaf0; border-radius: 10px; padding: 10px 12px; outline: none; }
      input:focus { border-color: rgba(99, 179, 237, 0.9); }
      .row { display: grid; grid-template-columns: 1fr; gap: 12px; }
      button { cursor: pointer; border: 1px solid rgba(255,255,255,0.18); background: rgba(255,255,255,0.10); color: #e7eaf0; padding: 10px 12px; border-radius: 10px; font-weight: 600; width: 100%; }
      button:hover { background: rgba(255,255,255,0.14); }
      .muted { opacity: 0.8; font-size: 12px; margin-top: 10px; }
      .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; }
      .captcha { display: inline-block; padding: 6px 10px; border-radius: 10px; border: 1px dashed rgba(255,255,255,0.25); background: rgba(255,255,255,0.06); }
      .err { margin: 0 0 12px; padding: 10px 12px; border-radius: 10px; border: 1px solid rgba(254, 202, 202, 0.25); background: rgba(254, 202, 202, 0.10); color: #fecaca; font-size: 12px; }
    </style>
  </head>
  <body>
    <div class="wrap">
      <h1>下单测试 — 登录</h1>
      <div class="card">
        ` + errMsg + `
        <form method="POST" action="/ui/login">
          <div class="row">
            <div>
              <label>账号</label>
              <input name="username" autocomplete="username" />
            </div>
            <div>
              <label>密码</label>
              <input name="password" type="password" autocomplete="current-password" />
            </div>
            <div>
              <label>验证码（输入下方数字）</label>
              <div class="muted">验证码：<span class="captcha mono">` + htmlEscape(code) + `</span></div>
              <input name="captcha" inputmode="numeric" />
              <input type="hidden" name="captcha_code" value="` + htmlEscape(code) + `" />
              <input type="hidden" name="captcha_token" value="` + htmlEscape(token) + `" />
            </div>
            <div>
              <button type="submit">登录</button>
            </div>
          </div>
        </form>
        <div class="muted">登录成功后自动跳转到下单页面</div>
      </div>
    </div>
  </body>
</html>`
}

func uiHMACHex(secret []byte, data string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func uiRandDigits(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return strconv.FormatInt(time.Now().UnixNano()%int64Pow10(n), 10)
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = byte('0' + (b[i] % 10))
	}
	return string(out)
}

func int64Pow10(n int) int64 {
	x := int64(1)
	for i := 0; i < n; i++ {
		x *= 10
	}
	return x
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

const orderTestHTML = `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>下单测试</title>
    <style>
      body { font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Arial; margin: 0; padding: 24px; background: #0b1020; color: #e7eaf0; }
      .wrap { max-width: 860px; margin: 0 auto; }
      h1 { margin: 0 0 16px; font-size: 20px; }
      .grid { display: grid; grid-template-columns: 1fr; gap: 12px; }
      .card { background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.10); border-radius: 12px; padding: 16px; }
      label { display: block; font-size: 12px; opacity: 0.9; margin-bottom: 6px; }
      input, textarea { width: 100%; box-sizing: border-box; background: rgba(255,255,255,0.08); border: 1px solid rgba(255,255,255,0.14); color: #e7eaf0; border-radius: 10px; padding: 10px 12px; outline: none; }
      input:focus, textarea:focus { border-color: rgba(99, 179, 237, 0.9); }
      textarea { min-height: 140px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; }
      .row { display: grid; grid-template-columns: 1fr; gap: 12px; }
      .btns { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 10px; }
      button { cursor: pointer; border: 1px solid rgba(255,255,255,0.18); background: rgba(255,255,255,0.10); color: #e7eaf0; padding: 10px 12px; border-radius: 10px; font-weight: 600; }
      button:hover { background: rgba(255,255,255,0.14); }
      .muted { opacity: 0.8; font-size: 12px; }
      .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; }
      .ok { color: #a7f3d0; }
      .bad { color: #fecaca; }
    </style>
  </head>
  <body>
    <div class="wrap">
      <h1>customize-teldog-api — 下单测试</h1>
      <div class="grid">
        <div class="card">
          <div class="row">
            <div>
              <label>代理订单号（agent_order_id）</label>
              <input id="agentOrderId" placeholder="例如：MY-1720000000000" />
            </div>
            <div>
              <label>商品编码（product_code）</label>
              <input id="productCode" placeholder="例如：my-xxx-30" />
            </div>
            <div>
              <label>手机号（英文逗号分隔多个号码）</label>
              <input id="phone" value="601124320985,601234567890" />
            </div>
          </div>

          <div class="btns">
            <button id="btnGen">生成订单号</button>
            <button id="btnOrder">提交下单</button>
          </div>

          <div class="muted" style="margin-top:10px;">
            <div>服务地址：<span class="mono" id="baseUrl"></span></div>
            <div>下单接口：<span class="mono">POST /api/teldog/orders</span></div>
          </div>
        </div>

        <div class="card">
          <div class="muted">返回结果</div>
          <div style="margin-top:10px;" class="mono" id="statusLine"></div>
          <textarea id="output" spellcheck="false"></textarea>
        </div>
      </div>
    </div>

    <script>
      const $ = (id) => document.getElementById(id);
      const base = location.origin;
      $("baseUrl").textContent = base;

      function normalizePhone(phone) {
        return String(phone || "").replace(/[^\d]/g, "");
      }

      function parsePhones(input) {
        return String(input || "")
          .split(",")
          .map((s) => normalizePhone(s))
          .filter((s) => s.length > 0);
      }

      function guessCountryPrefixFromProductCode(productCode) {
        const s = String(productCode || "").trim();
        if (!s) return "XX";
        const token = s.split(/[-_]/)[0] || "";
        if (/^[A-Za-z]{2}$/.test(token)) return token.toUpperCase();
        if (/^(my|malaysia)\b/i.test(token)) return "MY";
        if (/^(us|usa)\b/i.test(token)) return "US";
        if (/^(cn|china)\b/i.test(token)) return "CN";
        if (/^(sg|singapore)\b/i.test(token)) return "SG";
        if (/^(id|indonesia)\b/i.test(token)) return "ID";
        if (/^(th|thailand)\b/i.test(token)) return "TH";
        if (/^(vn|vietnam)\b/i.test(token)) return "VN";
        if (/^(ph|philippines)\b/i.test(token)) return "PH";
        return "XX";
      }

      function setOut(status, body) {
        const ok = status >= 200 && status < 300;
        $("statusLine").innerHTML = ok ? '<span class="ok">HTTP ' + status + '</span>' : '<span class="bad">HTTP ' + status + '</span>';
        if (typeof body === "string") {
          $("output").value = body;
          return;
        }
        try {
          $("output").value = JSON.stringify(body, null, 2);
        } catch {
          $("output").value = String(body);
        }
      }

      async function httpPost(path, payload) {
        const res = await fetch(base + path, {
          method: "POST",
          headers: { "Accept": "application/json", "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        const text = await res.text();
        let json;
        try { json = JSON.parse(text); } catch { json = text; }
        return { status: res.status, body: json };
      }

      $("btnGen").addEventListener("click", (e) => {
        e.preventDefault();
        const country = guessCountryPrefixFromProductCode($("productCode").value);
        $("agentOrderId").value = (country ? country : "XX") + "-" + Date.now();
      });

      $("btnOrder").addEventListener("click", async (e) => {
        e.preventDefault();
        const baseOrderIdInput = $("agentOrderId").value.trim();
        const productCode = $("productCode").value.trim();
        const phones = parsePhones($("phone").value);
        if (!productCode || phones.length === 0) {
          setOut(422, { code: 42201, message: "product_code 和 phone 为必填", data: {} });
          return;
        }

        const baseOrderId = baseOrderIdInput || ((guessCountryPrefixFromProductCode(productCode) || "XX") + "-" + Date.now());
        $("agentOrderId").value = baseOrderId;

        const results = [];
        for (let i = 0; i < phones.length; i++) {
          const phone = phones[i];
          const agentOrderId = phones.length === 1 ? baseOrderId : (baseOrderId + "-" + (i + 1));
          $("statusLine").innerHTML = '<span class="mono">提交中 ' + (i + 1) + '/' + phones.length + '</span>';
          const payload = { agent_order_id: agentOrderId, product_code: productCode, phone: phone };
          const res = await httpPost("/api/teldog/orders", payload);
          results.push({ phone: phone, agent_order_id: agentOrderId, http_status: res.status, response: res.body });
        }

        const allOK = results.every((r) => r.http_status >= 200 && r.http_status < 300);
        setOut(allOK ? 200 : 500, results);
      });
    </script>
  </body>
</html>`
