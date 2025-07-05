package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bjdgyc/anylink/base"
	"github.com/bjdgyc/anylink/dbdata"
	"github.com/gorilla/mux"
)

// OIDCSessionV5 OIDC会话数据
type OIDCSessionV5 struct {
	State           string    `json:"state"`
	Nonce          string    `json:"nonce"`
	RedirectURL    string    `json:"redirect_url"`
	CreatedAt      time.Time `json:"created_at"`
	GroupName      string    `json:"group_name"`
	ClientIP       string    `json:"client_ip"`
	UserAgent      string    `json:"user_agent"`
}

// LinkOidcV5Login V5版本OIDC登录入口
func LinkOidcV5Login(w http.ResponseWriter, r *http.Request) {
	base.Info("OIDC V5 登录请求")
	
	// 获取组名
	groupName := r.URL.Query().Get("group")
	if groupName == "" {
		base.Error("缺少组参数")
		http.Error(w, "缺少组参数", http.StatusBadRequest)
		return
	}
	
	// 获取组配置
	group := dbdata.GetGroup(groupName)
	if group == nil {
		base.Error("组不存在:", groupName)
		http.Error(w, "组配置不存在", http.StatusNotFound)
		return
	}
	
	// 验证是否为OIDC V5认证
	authType, ok := group.Auth["type"].(string)
	if !ok || authType != "oidc_v5" {
		base.Error("组认证类型不是OIDC V5:", authType)
		http.Error(w, "不支持的认证类型", http.StatusBadRequest)
		return
	}
	
	// 解析OIDC V5配置
	var authConfig dbdata.AuthOidcV5
	if err := parseAuthConfig(group.Auth, "oidc_v5", &authConfig); err != nil {
		base.Error("解析OIDC V5配置失败:", err)
		http.Error(w, "认证配置错误", http.StatusInternalServerError)
		return
	}
	
	// 创建OIDC客户端
	client, err := dbdata.NewOIDCClientV5(&authConfig)
	if err != nil {
		base.Error("创建OIDC客户端失败:", err)
		http.Error(w, "认证服务不可用", http.StatusInternalServerError)
		return
	}
	
	// 生成状态参数
	state, nonce, err := client.GenerateStateAndNonce()
	if err != nil {
		base.Error("生成状态参数失败:", err)
		http.Error(w, "认证参数生成失败", http.StatusInternalServerError)
		return
	}
	
	// 保存会话状态
	sessionData := &OIDCSessionV5{
		State:       state,
		Nonce:       nonce,
		RedirectURL: r.URL.Query().Get("redirect_url"),
		CreatedAt:   time.Now(),
		GroupName:   groupName,
		ClientIP:    getClientIP(r),
		UserAgent:   r.UserAgent(),
	}
	
	if err := saveOIDCSession(state, sessionData); err != nil {
		base.Error("保存会话状态失败:", err)
		http.Error(w, "会话创建失败", http.StatusInternalServerError)
		return
	}
	
	// 获取授权URL
	authURL := client.GetAuthCodeURL(state, nonce)
	
	base.Info("OIDC V5 重定向到授权页面:", authURL)
	
	// 重定向到授权页面
	http.Redirect(w, r, authURL, http.StatusFound)
}

// LinkOidcV5Callback V5版本OIDC回调处理
func LinkOidcV5Callback(w http.ResponseWriter, r *http.Request) {
	base.Info("OIDC V5 回调处理")
	
	// 获取回调参数
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorCode := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")
	
	// 检查错误
	if errorCode != "" {
		base.Error("OIDC授权失败:", errorCode, errorDesc)
		http.Error(w, fmt.Sprintf("授权失败: %s - %s", errorCode, errorDesc), http.StatusBadRequest)
		return
	}
	
	// 验证必需参数
	if code == "" || state == "" {
		base.Error("缺少必需的回调参数")
		http.Error(w, "缺少必需的回调参数", http.StatusBadRequest)
		return
	}
	
	// 获取会话状态
	sessionData, err := getOIDCSession(state)
	if err != nil {
		base.Error("获取会话状态失败:", err)
		http.Error(w, "会话已过期或无效", http.StatusBadRequest)
		return
	}
	
	// 验证状态参数
	if sessionData.State != state {
		base.Error("状态参数不匹配")
		http.Error(w, "状态参数不匹配", http.StatusBadRequest)
		return
	}
	
	// 检查会话是否过期
	if time.Since(sessionData.CreatedAt) > 10*time.Minute {
		base.Error("会话已过期")
		http.Error(w, "会话已过期", http.StatusBadRequest)
		return
	}
	
	// 获取组配置
	group := dbdata.GetGroup(sessionData.GroupName)
	if group == nil {
		base.Error("获取组配置失败:", sessionData.GroupName)
		http.Error(w, "组配置不存在", http.StatusNotFound)
		return
	}
	
	// 解析OIDC V5配置
	var authConfig dbdata.AuthOidcV5
	if err := parseAuthConfig(group.Auth, "oidc_v5", &authConfig); err != nil {
		base.Error("解析OIDC V5配置失败:", err)
		http.Error(w, "认证配置错误", http.StatusInternalServerError)
		return
	}
	
	// 创建OIDC客户端
	client, err := dbdata.NewOIDCClientV5(&authConfig)
	if err != nil {
		base.Error("创建OIDC客户端失败:", err)
		http.Error(w, "认证服务不可用", http.StatusInternalServerError)
		return
	}
	
	// 交换授权码获取令牌
	tokens, err := client.ExchangeCode(code, state)
	if err != nil {
		base.Error("交换授权码失败:", err)
		http.Error(w, "令牌获取失败", http.StatusInternalServerError)
		return
	}
	
	// 获取用户信息
	userInfo, err := client.GetUserInfo(tokens)
	if err != nil {
		base.Error("获取用户信息失败:", err)
		http.Error(w, "用户信息获取失败", http.StatusInternalServerError)
		return
	}
	
	// 验证用户权限
	if err := client.ValidateUser(userInfo); err != nil {
		base.Error("用户权限验证失败:", err)
		http.Error(w, "用户权限不足", http.StatusForbidden)
		return
	}
	
	// 获取用户名
	username := client.GetUsername(userInfo)
	if username == "" {
		base.Error("无法获取用户名")
		http.Error(w, "用户名获取失败", http.StatusInternalServerError)
		return
	}
	
	base.Info("OIDC V5 用户认证成功:", username)
	
	// 创建认证令牌
	authToken := "oidc_v5_token_" + generateRandomString(32)
	
	// 保存认证信息
	if err := saveOIDCAuthInfo(authToken, username, userInfo, tokens, sessionData.GroupName); err != nil {
		base.Error("保存认证信息失败:", err)
		http.Error(w, "认证信息保存失败", http.StatusInternalServerError)
		return
	}
	
	// 清理会话状态
	deleteOIDCSession(state)
	
	// 设置认证Cookie
	cookie := &http.Cookie{
		Name:     "oidc_v5_token",
		Value:    authToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   authConfig.SessionTimeout,
	}
	http.SetCookie(w, cookie)
	
	// 生成继续登录的参数
	continueParams := map[string]string{
		"username": username,
		"token":    authToken,
		"group":    sessionData.GroupName,
	}
	
	// 重定向到VPN客户端或Web界面
	if sessionData.RedirectURL != "" {
		// 自定义重定向URL
		redirectURL := buildRedirectURL(sessionData.RedirectURL, continueParams)
		http.Redirect(w, r, redirectURL, http.StatusFound)
	} else {
		// 默认重定向到AnyConnect兼容页面
		redirectURL := fmt.Sprintf("/CSCOE/sso-auth-complete?username=%s&token=%s&group=%s", 
			username, authToken, sessionData.GroupName)
		http.Redirect(w, r, redirectURL, http.StatusFound)
	}
}

// LinkOidcV5Token V5版本令牌验证
func LinkOidcV5Token(w http.ResponseWriter, r *http.Request) {
	base.Info("OIDC V5 令牌验证")
	
	// 获取令牌
	token := r.URL.Query().Get("token")
	if token == "" {
		// 尝试从Cookie获取
		if cookie, err := r.Cookie("oidc_v5_token"); err == nil {
			token = cookie.Value
		}
	}
	
	if token == "" {
		base.Error("缺少认证令牌")
		http.Error(w, "缺少认证令牌", http.StatusBadRequest)
		return
	}
	
	// 验证令牌
	authInfo, err := getOIDCAuthInfo(token)
	if err != nil {
		base.Error("令牌验证失败:", err)
		http.Error(w, "令牌无效", http.StatusUnauthorized)
		return
	}
	
	// 检查令牌是否过期
	if time.Since(authInfo.CreatedAt) > time.Duration(authInfo.SessionTimeout)*time.Second {
		base.Error("令牌已过期")
		deleteOIDCAuthInfo(token)
		http.Error(w, "令牌已过期", http.StatusUnauthorized)
		return
	}
	
	// 返回用户信息
	response := map[string]interface{}{
		"username": authInfo.Username,
		"email":    authInfo.Email,
		"groups":   authInfo.Groups,
		"roles":    authInfo.Roles,
		"valid":    true,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// LinkOidcV5Logout V5版本退出登录
func LinkOidcV5Logout(w http.ResponseWriter, r *http.Request) {
	base.Info("OIDC V5 退出登录")
	
	// 获取令牌
	token := r.URL.Query().Get("token")
	if token == "" {
		if cookie, err := r.Cookie("oidc_v5_token"); err == nil {
			token = cookie.Value
		}
	}
	
	// 清理认证信息
	if token != "" {
		deleteOIDCAuthInfo(token)
	}
	
	// 清理Cookie
	cookie := &http.Cookie{
		Name:     "oidc_v5_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, cookie)
	
	// 获取登出重定向URL
	redirectURL := r.URL.Query().Get("redirect_url")
	if redirectURL == "" {
		redirectURL = "/"
	}
	
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// LinkOidcV5Refresh V5版本刷新令牌
func LinkOidcV5Refresh(w http.ResponseWriter, r *http.Request) {
	base.Info("OIDC V5 刷新令牌")
	
	// 获取刷新令牌
	var requestData struct {
		RefreshToken string `json:"refresh_token"`
		Token        string `json:"token"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		base.Error("解析请求数据失败:", err)
		http.Error(w, "请求数据格式错误", http.StatusBadRequest)
		return
	}
	
	// 验证当前令牌
	authInfo, err := getOIDCAuthInfo(requestData.Token)
	if err != nil {
		base.Error("令牌验证失败:", err)
		http.Error(w, "令牌无效", http.StatusUnauthorized)
		return
	}
	
	// 获取组配置
	group := dbdata.GetGroup(authInfo.GroupName)
	if group == nil {
		base.Error("获取组配置失败:", authInfo.GroupName)
		http.Error(w, "组配置不存在", http.StatusNotFound)
		return
	}
	
	// 解析OIDC V5配置
	var authConfig dbdata.AuthOidcV5
	if err := parseAuthConfig(group.Auth, "oidc_v5", &authConfig); err != nil {
		base.Error("解析OIDC V5配置失败:", err)
		http.Error(w, "认证配置错误", http.StatusInternalServerError)
		return
	}
	
	// 创建OIDC客户端
	client, err := dbdata.NewOIDCClientV5(&authConfig)
	if err != nil {
		base.Error("创建OIDC客户端失败:", err)
		http.Error(w, "认证服务不可用", http.StatusInternalServerError)
		return
	}
	
	// 刷新令牌
	newTokens, err := client.RefreshTokens(requestData.RefreshToken)
	if err != nil {
		base.Error("刷新令牌失败:", err)
		http.Error(w, "令牌刷新失败", http.StatusInternalServerError)
		return
	}
	
	// 获取更新的用户信息
	userInfo, err := client.GetUserInfo(newTokens)
	if err != nil {
		base.Error("获取用户信息失败:", err)
		http.Error(w, "用户信息获取失败", http.StatusInternalServerError)
		return
	}
	
	// 验证用户权限
	if err := client.ValidateUser(userInfo); err != nil {
		base.Error("用户权限验证失败:", err)
		http.Error(w, "用户权限不足", http.StatusForbidden)
		return
	}
	
	// 创建新的认证令牌
	newAuthToken := "oidc_v5_token_" + generateRandomString(32)
	
	// 保存新的认证信息
	if err := saveOIDCAuthInfo(newAuthToken, authInfo.Username, userInfo, newTokens, authInfo.GroupName); err != nil {
		base.Error("保存认证信息失败:", err)
		http.Error(w, "认证信息保存失败", http.StatusInternalServerError)
		return
	}
	
	// 清理旧的认证信息
	deleteOIDCAuthInfo(requestData.Token)
	
	// 返回新令牌
	response := map[string]interface{}{
		"token":         newAuthToken,
		"access_token":  newTokens.AccessToken,
		"refresh_token": newTokens.RefreshToken,
		"expires_at":    newTokens.ExpiresAt,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// 辅助函数

// parseAuthConfig 解析认证配置
func parseAuthConfig(authData map[string]interface{}, authType string, config interface{}) error {
	authTypeData, ok := authData[authType]
	if !ok {
		return fmt.Errorf("找不到认证类型 %s 的配置", authType)
	}
	
	configBytes, err := json.Marshal(authTypeData)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}
	
	return json.Unmarshal(configBytes, config)
}

// getClientIP 获取客户端IP
func getClientIP(r *http.Request) string {
	// 检查X-Forwarded-For头
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// 检查X-Real-IP头
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// 使用RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	
	return ip
}

// buildRedirectURL 构建重定向URL
func buildRedirectURL(baseURL string, params map[string]string) string {
	if len(params) == 0 {
		return baseURL
	}
	
	separator := "?"
	if strings.Contains(baseURL, "?") {
		separator = "&"
	}
	
	var paramPairs []string
	for k, v := range params {
		paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", k, v))
	}
	
	return baseURL + separator + strings.Join(paramPairs, "&")
}

// 注册路由
func init() {
	// 注册OIDC V5路由
	authRouter := mux.NewRouter().PathPrefix("/oidc/v5").Subrouter()
	
	authRouter.HandleFunc("/login", LinkOidcV5Login).Methods("GET")
	authRouter.HandleFunc("/callback", LinkOidcV5Callback).Methods("GET")
	authRouter.HandleFunc("/token", LinkOidcV5Token).Methods("GET")
	authRouter.HandleFunc("/logout", LinkOidcV5Logout).Methods("GET", "POST")
	authRouter.HandleFunc("/refresh", LinkOidcV5Refresh).Methods("POST")
	
	// 兼容旧路由
	authRouter.HandleFunc("/auth", LinkOidcV5Login).Methods("GET")
	authRouter.HandleFunc("/verify", LinkOidcV5Token).Methods("GET")
}