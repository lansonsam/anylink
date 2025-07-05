package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bjdgyc/anylink/base"
	"github.com/bjdgyc/anylink/dbdata"
	"github.com/bjdgyc/anylink/pkg/utils"
	"github.com/bjdgyc/anylink/sessdata"
)

// OIDC 登录处理
func LinkOidcLogin(w http.ResponseWriter, r *http.Request) {
	// 获取组信息 - 支持多种参数格式
	groupName := r.URL.Query().Get("group")
	ctx := r.URL.Query().Get("ctx")
	
	// 如果是从Cisco标准路由来的，尝试从会话上下文获取组信息
	if groupName == "" && ctx != "" {
		// 从会话上下文获取组信息
		tempToken := sessdata.GetTempToken("ctx_" + ctx)
		if tempToken != nil && tempToken.GroupName != "" {
			groupName = tempToken.GroupName
			base.Info("OIDC Login - Retrieved group from context:", groupName)
		} else {
			// 如果没有上下文，返回错误而不是使用默认组
			http.Error(w, "缺少组信息，请重新选择组", http.StatusBadRequest)
			return
		}
	}
	
	if groupName == "" {
		http.Error(w, "缺少组参数", http.StatusBadRequest)
		return
	}

	group := dbdata.GetGroup(groupName)
	if group == nil {
		http.Error(w, "组不存在", http.StatusNotFound)
		return
	}

	// 添加调试信息
	base.Info("OIDC Login - Group:", groupName, "Auth config:", group.Auth)

	// 检查是否为 OIDC 认证
	authType, ok := group.Auth["type"].(string)
	if !ok || authType != "oidc" {
		base.Error("OIDC auth check failed - Auth:", group.Auth, "Type:", authType, "OK:", ok)
		http.Error(w, "组未配置 OIDC 认证", http.StatusBadRequest)
		return
	}

	// 解析 OIDC 配置
	authConfigJson, _ := json.Marshal(group.Auth["oidc"])
	var oidcAuth dbdata.AuthOidc
	if err := json.Unmarshal(authConfigJson, &oidcAuth); err != nil {
		base.Error("解析 OIDC 配置失败:", err)
		http.Error(w, "OIDC 配置错误", http.StatusInternalServerError)
		return
	}

	// 生成状态码和授权 URL
	state := oidcAuth.GenerateState()
	authURL, err := oidcAuth.GetAuthURL(state)
	if err != nil {
		base.Error("生成 OIDC 授权 URL 失败:", err)
		http.Error(w, "生成授权 URL 失败", http.StatusInternalServerError)
		return
	}

	// 存储状态码（实际项目中应该存储在缓存中）
	tempToken := &sessdata.TempTokenData{
		Token:     state,
		GroupName: groupName,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Data:      map[string]interface{}{"type": "oidc_state"},
	}
	sessdata.StoreTempToken(tempToken)

	// 重定向到 OIDC 提供商
	http.Redirect(w, r, authURL, http.StatusFound)
}

// OIDC 回调处理
func LinkOidcCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		base.Error("OIDC 认证错误:", errorParam)
		http.Error(w, "认证失败: "+errorParam, http.StatusBadRequest)
		return
	}

	if code == "" || state == "" {
		http.Error(w, "缺少授权码或状态参数", http.StatusBadRequest)
		return
	}

	// 验证状态码
	tempToken := sessdata.GetTempToken(state)
	if tempToken == nil {
		http.Error(w, "无效的状态码", http.StatusBadRequest)
		return
	}

	groupName := tempToken.GroupName
	group := dbdata.GetGroup(groupName)
	if group == nil {
		http.Error(w, "组不存在", http.StatusNotFound)
		return
	}

	// 解析 OIDC 配置
	authConfigJson, _ := json.Marshal(group.Auth["oidc"])
	var oidcAuth dbdata.AuthOidc
	if err := json.Unmarshal(authConfigJson, &oidcAuth); err != nil {
		base.Error("解析 OIDC 配置失败:", err)
		http.Error(w, "OIDC 配置错误", http.StatusInternalServerError)
		return
	}

	// 交换授权码获取令牌
	tokenResp, err := oidcAuth.ExchangeCodeForToken(code, state)
	if err != nil {
		base.Error("交换 OIDC 令牌失败:", err)
		http.Error(w, "令牌交换失败", http.StatusInternalServerError)
		return
	}

	// 获取用户信息
	userInfo, err := oidcAuth.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		base.Error("获取 OIDC 用户信息失败:", err)
		http.Error(w, "获取用户信息失败", http.StatusInternalServerError)
		return
	}

	// 解析 ID Token
	var idTokenClaims map[string]interface{}
	if tokenResp.IdToken != "" {
		idTokenClaims, err = oidcAuth.ParseIdToken(tokenResp.IdToken)
		if err != nil {
			base.Warn("解析 ID Token 失败:", err)
			idTokenClaims = make(map[string]interface{})
		}
	}

	// 验证用户
	username, err := oidcAuth.ValidateUser(userInfo, idTokenClaims)
	if err != nil {
		base.Error("OIDC 用户验证失败:", err)
		http.Error(w, "用户验证失败: "+err.Error(), http.StatusForbidden)
		return
	}

	// 生成会话令牌
	sessionToken := "oidc_token_" + sessdata.GenerateSessionToken()
	sessionData := &sessdata.TempTokenData{
		Token:     sessionToken,
		GroupName: groupName,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Data: map[string]interface{}{
			"type":     "oidc_session",
			"username": username,
			"email":    userInfo.Email,
		},
	}
	sessdata.StoreTempToken(sessionData)

	// 删除状态令牌
	sessdata.DeleteTempToken(state)

	// 设置 Cookie 而不是显示令牌页面
	// 从完整令牌中提取纯令牌部分 (移除 "oidc_token_" 前缀)
	pureToken := sessionToken
	if strings.HasPrefix(sessionToken, "oidc_token_") {
		pureToken = sessionToken[11:] // 移除 "oidc_token_" 前缀
	}
	
	SetCookie(w, "acSamlv2Token", pureToken, 0)  // 使用官方Cookie名称
	SetCookie(w, "acSamlv2Error", "", 0)         // 清除错误cookie
	
	// 重定向到正版格式的登录完成页面
	redirectURL := "/+CSCOE+/saml_ac_login.html"
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// OIDC 令牌验证
func LinkOidcToken(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.FormValue("token")
	}
	if token == "" {
		http.Error(w, "缺少令牌", http.StatusBadRequest)
		return
	}

	// 移除 Bearer 前缀
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// 验证令牌
	sessionData := sessdata.GetTempToken(token)
	if sessionData == nil {
		http.Error(w, "无效的令牌", http.StatusUnauthorized)
		return
	}

	if sessionData.Data["type"] != "oidc_session" {
		http.Error(w, "令牌类型错误", http.StatusUnauthorized)
		return
	}

	// 返回用户信息
	response := map[string]interface{}{
		"username":  sessionData.Data["username"],
		"group":     sessionData.GroupName,
		"email":     sessionData.Data["email"],
		"expires_at": sessionData.ExpiresAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SSO 完成端点处理
func LinkOidcSsoComplete(w http.ResponseWriter, r *http.Request) {
	sessionToken := r.URL.Query().Get("session-token")
	
	// 验证会话令牌
	sessionData := sessdata.GetTempToken(sessionToken)
	if sessionData == nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}
	
	if sessionData.Data["type"] != "oidc_session" {
		http.Error(w, "Invalid session type", http.StatusUnauthorized)
		return
	}
	
	// 为所有用户（包括AnyConnect客户端）准备VPN认证
	username := sessionData.Data["username"].(string)
	groupName := sessionData.GroupName
	
	// 验证VPN用户
	vpnUser := &dbdata.User{}
	err := dbdata.One("Username", username, vpnUser)
	if err != nil {
		http.Error(w, "User not found in VPN system", http.StatusUnauthorized)
		return
	}
	
	// 检查用户状态和权限
	if vpnUser.Status != 1 {
		http.Error(w, "User account is disabled", http.StatusUnauthorized)
		return
	}
	
	if !utils.InArrStr(vpnUser.Groups, groupName) {
		http.Error(w, "User not authorized for this group", http.StatusUnauthorized)
		return
	}
	
	// 创建VPN会话令牌
	vpnToken := "vpn_session_" + sessdata.GenerateSessionToken()
	vpnSessionData := &sessdata.TempTokenData{
		Token:     vpnToken,
		GroupName: groupName,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Data: map[string]interface{}{
			"type":     "vpn_session",
			"username": username,
			"authenticated": true,
		},
	}
	sessdata.StoreTempToken(vpnSessionData)
	
	// 设置VPN令牌到Cookie中（按照官方模式）
	SetCookie(w, "acSamlv2Token", vpnToken[12:], 0) // 移除"vpn_session_"前缀
	SetCookie(w, "acSamlv2Error", "", 0)
	
	// 删除OIDC会话令牌
	sessdata.DeleteTempToken(sessionToken)
	
	// 按照官方模式：设置Cookie并重定向到logon.html
	// 这样AnyConnect客户端会自动调用logon.html并获取配置
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	html := `<html>
<head>
<script>
document.location.replace("/+CSCOE+/logon.html?" +
"a0=0" +
"&a1=" +
"&a2=" +
"&a3=1");
</script>
</head>
</html>`
	w.Write([]byte(html))
	return
}

// +CSCOE+/logon.html 端点处理 - 按照官方模式
func LinkCscoeLogon(w http.ResponseWriter, r *http.Request) {
	// 检查User-Agent以确定是否为AnyConnect客户端
	userAgent := r.Header.Get("User-Agent")
	isAnyConnectClient := strings.Contains(userAgent, "AnyConnect")
	
	if isAnyConnectClient {
		// 从Cookie获取VPN令牌
		cookie, err := r.Cookie("acSamlv2Token")
		if err != nil || cookie.Value == "" {
			// 没有有效的Cookie，返回认证请求
			xmlResponse := `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="auth-request" aggregate-auth-version="2">
	<auth id="main">
		<title>Authentication Required</title>
		<message>Please complete authentication</message>
	</auth>
</config-auth>`
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			w.Write([]byte(xmlResponse))
			return
		}
		
		// 验证VPN会话令牌
		vpnToken := "vpn_session_" + cookie.Value
		sessionData := sessdata.GetTempToken(vpnToken)
		if sessionData == nil || sessionData.Data["type"] != "vpn_session" {
			// 令牌无效，返回认证请求
			xmlResponse := `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="auth-request" aggregate-auth-version="2">
	<auth id="main">
		<title>Authentication Required</title>
		<message>Session expired, please authenticate again</message>
	</auth>
</config-auth>`
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			w.Write([]byte(xmlResponse))
			return
		}
		
		// 令牌有效，返回认证完成的XML
		username := sessionData.Data["username"].(string)
		groupName := sessionData.GroupName
		
		xmlResponse := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="complete" aggregate-auth-version="2">
	<opaque is-for="sg">
		<tunnel-group>%s</tunnel-group>
		<group-alias>%s</group-alias>
		<config-hash>1595829378234</config-hash>
		<auth-method>single-sign-on-v2</auth-method>
	</opaque>
	<auth id="success">
		<title>Login Successful</title>
		<message>VPN Authentication completed for %s</message>
		<banner></banner>
	</auth>
	<config>
		<vpn-tunnel-protocol>ssl-client</vpn-tunnel-protocol>
		<ssl-tunnel-protocol>sslv3</ssl-tunnel-protocol>
		<auto-signon>
			<tunnel-group>%s</tunnel-group>
			<auth-type>2</auth-type>
		</auto-signon>
	</config>
</config-auth>`, groupName, groupName, username, groupName)
		
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store,no-cache")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("X-Aggregate-Auth", "1")
		w.Write([]byte(xmlResponse))
		return
	}
	
	// 为浏览器用户返回HTML页面
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Secure Client</title>
    <meta charset="utf-8">
    <style>
        body {
            font-family: "Cisco Sans", "Helvetica Neue", Helvetica, Arial, sans-serif;
            margin: 0;
            padding: 0;
            background-color: #f5f5f5;
        }
        .header {
            background-color: #0096D6;
            color: white;
            padding: 15px 20px;
            display: flex;
            align-items: center;
        }
        .cisco-logo {
            font-weight: bold;
            font-size: 16px;
            margin-right: 20px;
        }
        .title {
            font-size: 18px;
            font-weight: normal;
        }
        .content {
            max-width: 600px;
            margin: 40px auto;
            padding: 0 20px;
        }
        .success-message {
            background-color: #d4edda;
            border: 1px solid #c3e6cb;
            border-radius: 4px;
            color: #155724;
            padding: 15px;
            margin-bottom: 20px;
            display: flex;
            align-items: center;
        }
        .success-icon {
            color: #28a745;
            font-size: 20px;
            margin-right: 10px;
        }
        .success-text {
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="cisco-logo">🏢 cisco</div>
        <div class="title">Secure Client</div>
    </div>
    <div class="content">
        <div class="success-message">
            <div class="success-icon">✓</div>
            <div class="success-text">You have successfully authenticated. You may now close this browser tab.</div>
        </div>
    </div>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

