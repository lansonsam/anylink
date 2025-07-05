package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// OIDC配置
type OpenIDConfiguration struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JwksURI               string `json:"jwks_uri"`
}

// 令牌响应
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IdToken     string `json:"id_token"`
}

// 用户信息
type UserInfo struct {
	Sub               string   `json:"sub"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	Groups            []string `json:"groups"`
}

// 简单的内存存储
var (
	authCodes    = make(map[string]AuthCode)
	accessTokens = make(map[string]UserInfo)
	qqSessions   = make(map[string]*QQSession) // QQ登录会话
	qqMutex      sync.RWMutex
	users        = map[string]UserInfo{
		"admin": {
			Sub:               "admin",
			PreferredUsername: "admin",
			Email:             "admin@test.com",
			Name:              "Administrator",
			Groups:            []string{"ops", "all"},
		},
		"user1": {
			Sub:               "user1", 
			PreferredUsername: "user1",
			Email:             "user1@test.com",
			Name:              "Test User 1",
			Groups:            []string{"all"},
		},
	}
)

type AuthCode struct {
	Code        string
	ClientID    string
	RedirectURI string
	UserInfo    UserInfo
	ExpiresAt   time.Time
}

// QQ登录会话
type QQSession struct {
	SessionID    string
	State        string
	QQNumber     string
	Status       string // pending, scanning, confirmed, completed, failed
	CreatedAt    time.Time
	ExpiresAt    time.Time
	UserInfo     *UserInfo
	ErrorMessage string
}

func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// OIDC配置端点
func wellKnownHandler(w http.ResponseWriter, r *http.Request) {
	config := OpenIDConfiguration{
		Issuer:                "https://aaa.ai520.me:8080",
		AuthorizationEndpoint: "https://aaa.ai520.me:8080/auth",
		TokenEndpoint:         "https://aaa.ai520.me:8080/token",
		UserinfoEndpoint:      "https://aaa.ai520.me:8080/userinfo",
		JwksURI:               "https://aaa.ai520.me:8080/jwks",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// QQ登录处理函数
func qqLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// 创建QQ登录会话
		sessionID := generateRandomString(32)
		qqSession := &QQSession{
			SessionID: sessionID,
			State:     r.URL.Query().Get("state"),
			Status:    "pending",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		
		qqMutex.Lock()
		qqSessions[sessionID] = qqSession
		qqMutex.Unlock()
		
		// 启动Python脚本生成二维码
		go generateQQQRCode(sessionID)
		
		// 返回QQ登录页面
		qqLoginPage := `
<!DOCTYPE html>
<html>
<head>
    <title>QQ登录</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; max-width: 400px; margin: 100px auto; padding: 20px; }
        .qr-container { text-align: center; margin: 20px 0; }
        .qr-container img { max-width: 200px; border: 1px solid #ddd; padding: 10px; }
        .status { text-align: center; margin: 20px 0; padding: 15px; background: #f0f0f0; border-radius: 5px; }
        .loading { color: #666; }
        .success { color: green; }
        .error { color: red; }
        h2 { text-align: center; color: #333; }
    </style>
</head>
<body>
    <h2>QQ扫码登录</h2>
    <div class="qr-container">
        <img id="qrcode" src="/qq/qrcode/{{.SessionID}}" alt="QQ登录二维码" onerror="this.src='/qq/qrcode/{{.SessionID}}?t=' + Date.now()">
        <p style="margin-top: 10px; font-size: 12px; color: #666;">
            如果二维码未显示，<a href="/qq/qrcode/{{.SessionID}}" target="_blank">点击这里查看</a>
        </p>
    </div>
    <div class="status" id="status">
        <div class="loading">请使用手机QQ扫描二维码登录</div>
    </div>
    
    <script>
    const sessionID = '{{.SessionID}}';
    const state = '{{.State}}';
    let checkInterval;
    
    function checkStatus() {
        fetch('/qq/status/' + sessionID)
            .then(resp => resp.json())
            .then(data => {
                const statusDiv = document.getElementById('status');
                
                switch(data.status) {
                    case 'scanning':
                        statusDiv.innerHTML = '<div class="loading">二维码已扫描，请在手机上确认登录</div>';
                        break;
                    case 'confirmed':
                        statusDiv.innerHTML = '<div class="loading">正在验证群组信息...</div>';
                        break;
                    case 'completed':
                        statusDiv.innerHTML = '<div class="success">登录成功！正在跳转...</div>';
                        clearInterval(checkInterval);
                        // 跳转回授权端点
                        window.location.href = '/auth?qq_session=' + sessionID + '&state=' + state;
                        break;
                    case 'failed':
                        statusDiv.innerHTML = '<div class="error">登录失败: ' + (data.error || '未知错误') + '</div>';
                        clearInterval(checkInterval);
                        break;
                    case 'expired':
                        statusDiv.innerHTML = '<div class="error">二维码已过期，请刷新页面重试</div>';
                        clearInterval(checkInterval);
                        break;
                }
            })
            .catch(err => {
                console.error('检查状态失败:', err);
            });
    }
    
    // 等待二维码生成
    setTimeout(() => {
        // 刷新二维码图片
        document.getElementById('qrcode').src = '/qq/qrcode/' + sessionID + '?t=' + Date.now();
    }, 1000);
    
    // 每2秒检查一次状态
    checkInterval = setInterval(checkStatus, 2000);
    checkStatus(); // 立即检查一次
    </script>
</body>
</html>`
		
		tmpl, _ := template.New("qqlogin").Parse(qqLoginPage)
		data := struct {
			SessionID string
			State     string
		}{
			SessionID: sessionID,
			State:     qqSession.State,
		}
		tmpl.Execute(w, data)
	}
}

// 生成QQ二维码
func generateQQQRCode(sessionID string) {
	fmt.Printf("开始生成QQ二维码，会话ID: %s\n", sessionID)
	
	// 获取当前工作目录
	pwd, _ := os.Getwd()
	fmt.Printf("当前工作目录: %s\n", pwd)
	
	// 调用Python脚本生成二维码
	cmd := exec.Command("python3", "qq group/qq_login_server.py", sessionID)
	cmd.Dir = "." // 设置工作目录
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	
	qqMutex.Lock()
	defer qqMutex.Unlock()
	
	if session, ok := qqSessions[sessionID]; ok {
		if err != nil {
			session.Status = "failed"
			session.ErrorMessage = "二维码生成失败"
		} else {
			// Python脚本会更新状态
		}
	}
}

// QQ二维码图片端点
func qqQRCodeHandler(w http.ResponseWriter, r *http.Request) {
	// sessionID := strings.TrimPrefix(r.URL.Path, "/qq/qrcode/")
	// 暂时不使用sessionID，所有用户共享一个二维码
	
	// 返回二维码图片
	qrFile := "qq_login_qr.png"
	if _, err := os.Stat(qrFile); err == nil {
		// 设置正确的Content-Type
		w.Header().Set("Content-Type", "image/png")
		http.ServeFile(w, r, qrFile)
	} else {
		fmt.Printf("二维码文件未找到: %s, 错误: %v\n", qrFile, err)
		http.Error(w, "二维码未生成", http.StatusNotFound)
	}
}

// QQ登录状态检查端点
func qqStatusHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/qq/status/")
	
	qqMutex.RLock()
	session, ok := qqSessions[sessionID]
	qqMutex.RUnlock()
	
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not_found",
			"error":  "会话不存在",
		})
		return
	}
	
	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "expired",
			"error":  "会话已过期",
		})
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": session.Status,
		"error":  session.ErrorMessage,
	})
}

// QQ验证回调端点（Python脚本调用）
func qqCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var data struct {
		SessionID string   `json:"session_id"`
		Status    string   `json:"status"`
		QQNumber  string   `json:"qq_number"`
		Nickname  string   `json:"nickname"` // QQ昵称
		Error     string   `json:"error"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	qqMutex.Lock()
	defer qqMutex.Unlock()
	
	session, ok := qqSessions[data.SessionID]
	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	
	session.Status = data.Status
	session.QQNumber = data.QQNumber
	
	if data.Error != "" {
		session.ErrorMessage = data.Error
	} else if data.Status == "completed" {
		// 创建用户信息
		name := data.Nickname
		if name == "" {
			name = "QQ用户" + data.QQNumber
		}
		session.UserInfo = &UserInfo{
			Sub:               "qq_" + data.QQNumber,
			PreferredUsername: "qq_" + data.QQNumber,
			Email:             data.QQNumber + "@qq.com",
			Name:              name,
			Groups:            []string{"all"}, // 默认分配到all组
		}
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// 授权端点
func authHandler(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	qqSessionID := r.URL.Query().Get("qq_session")
	
	// 如果有QQ会话ID，检查登录状态
	if qqSessionID != "" {
		qqMutex.RLock()
		qqSession, ok := qqSessions[qqSessionID]
		qqMutex.RUnlock()
		
		if ok && qqSession.Status == "completed" && qqSession.UserInfo != nil {
			// QQ登录成功，生成授权码
			code := generateRandomString(32)
			authCodes[code] = AuthCode{
				Code:        code,
				ClientID:    clientID,
				RedirectURI: redirectURI,
				UserInfo:    *qqSession.UserInfo,
				ExpiresAt:   time.Now().Add(10 * time.Minute),
			}
			
			// 清理QQ会话
			qqMutex.Lock()
			delete(qqSessions, qqSessionID)
			qqMutex.Unlock()
			
			// 重定向回客户端
			redirectURL, _ := url.Parse(redirectURI)
			params := redirectURL.Query()
			params.Set("code", code)
			params.Set("state", state)
			redirectURL.RawQuery = params.Encode()
			
			http.Redirect(w, r, redirectURL.String(), http.StatusFound)
			return
		}
	}
	
	if r.Method == "GET" {
		// 显示登录页面
		loginPage := `
<!DOCTYPE html>
<html>
<head>
    <title>OIDC 测试登录</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; max-width: 400px; margin: 100px auto; padding: 20px; }
        input, select, button { width: 100%; padding: 10px; margin: 10px 0; box-sizing: border-box; }
        button { background: #007cba; color: white; border: none; cursor: pointer; }
        button:hover { background: #005a87; }
        .form-group { margin: 15px 0; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
    </style>
</head>
<body>
    <h2>OIDC 测试登录</h2>
    <form method="post">
        <input type="hidden" name="client_id" value="{{.ClientID}}">
        <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
        <input type="hidden" name="state" value="{{.State}}">
        
        <div class="form-group">
            <label>用户名:</label>
            <select name="username" required>
                <option value="">请选择用户</option>
                <option value="admin">admin (管理员)</option>
                <option value="user1">user1 (普通用户)</option>
                <option value="qq">使用QQ登录</option>
            </select>
        </div>
        
        <div class="form-group">
            <label>密码:</label>
            <input type="password" name="password" placeholder="任意密码" required>
        </div>
        
        <button type="submit">登录</button>
    </form>
    
    <script>
    document.querySelector('form').addEventListener('submit', function(e) {
        const username = document.querySelector('select[name="username"]').value;
        if (username === 'qq') {
            e.preventDefault();
            // 跳转到QQ登录
            const params = new URLSearchParams(window.location.search);
            window.location.href = '/qq/login?' + params.toString();
        }
    });
    </script>
    
    <div style="margin-top: 30px; padding: 15px; background: #f0f0f0; border-radius: 5px;">
        <h3>测试说明:</h3>
        <p><strong>Client ID:</strong> {{.ClientID}}</p>
        <p><strong>Redirect URI:</strong> {{.RedirectURI}}</p>
        <p><strong>State:</strong> {{.State}}</p>
        <p>可用用户: admin, user1 (密码任意)</p>
    </div>
</body>
</html>`
		
		tmpl, _ := template.New("login").Parse(loginPage)
		data := struct {
			ClientID    string
			RedirectURI string
			State       string
		}{
			ClientID:    clientID,
			RedirectURI: redirectURI,
			State:       state,
		}
		tmpl.Execute(w, data)
		return
	}
	
	// 处理登录表单提交
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")
		clientID := r.FormValue("client_id")
		redirectURI := r.FormValue("redirect_uri")
		state := r.FormValue("state")
		
		// 简单验证（这里只检查用户是否存在，不验证密码）
		userInfo, exists := users[username]
		if !exists || password == "" {
			http.Error(w, "用户名或密码错误", http.StatusUnauthorized)
			return
		}
		
		// 生成授权码
		code := generateRandomString(32)
		authCodes[code] = AuthCode{
			Code:        code,
			ClientID:    clientID,
			RedirectURI: redirectURI,
			UserInfo:    userInfo,
			ExpiresAt:   time.Now().Add(10 * time.Minute),
		}
		
		// 重定向回客户端
		redirectURL, _ := url.Parse(redirectURI)
		params := redirectURL.Query()
		params.Set("code", code)
		params.Set("state", state)
		redirectURL.RawQuery = params.Encode()
		
		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
		return
	}
}

// 令牌端点
func tokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	grantType := r.FormValue("grant_type")
	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	
	// 验证客户端凭据（简化处理）
	if clientSecret != "secret" {
		http.Error(w, "客户端密钥错误", http.StatusUnauthorized)
		return
	}
	
	if grantType != "authorization_code" {
		http.Error(w, "不支持的授权类型", http.StatusBadRequest)
		return
	}
	
	// 验证授权码
	authCode, exists := authCodes[code]
	if !exists || authCode.ExpiresAt.Before(time.Now()) {
		http.Error(w, "无效的授权码", http.StatusBadRequest)
		return
	}
	
	if authCode.ClientID != clientID {
		http.Error(w, "客户端ID不匹配", http.StatusBadRequest)
		return
	}
	
	// 生成访问令牌
	accessToken := generateRandomString(32)
	accessTokens[accessToken] = authCode.UserInfo
	
	// 删除已使用的授权码
	delete(authCodes, code)
	
	// 返回令牌
	response := TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		IdToken:     generateRandomString(64), // 简化处理，实际应该是JWT
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// 用户信息端点
func userinfoHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "无效的授权头", http.StatusUnauthorized)
		return
	}
	
	accessToken := authHeader[7:]
	userInfo, exists := accessTokens[accessToken]
	if !exists {
		http.Error(w, "无效的访问令牌", http.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// JWKS端点（简化处理）
func jwksHandler(w http.ResponseWriter, r *http.Request) {
	jwks := map[string]interface{}{
		"keys": []interface{}{},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

// 首页，显示配置信息
func homeHandler(w http.ResponseWriter, r *http.Request) {
	homePage := `
<!DOCTYPE html>
<html>
<head>
    <title>OIDC 测试提供商</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 20px auto; padding: 20px; }
        .endpoint { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .config { background: #e8f4f8; padding: 15px; margin: 20px 0; border-radius: 5px; }
        code { background: #f0f0f0; padding: 2px 5px; border-radius: 3px; }
        pre { background: #f8f8f8; padding: 10px; border-radius: 5px; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>OIDC 测试提供商</h1>
    <p>这是一个简单的OIDC提供商，用于测试AnyLink的OIDC集成。</p>
    
    <div class="config">
        <h2>配置信息</h2>
        <p><strong>Issuer URL:</strong> <code>https://aaa.ai520.me:8080</code></p>
        <p><strong>Client ID:</strong> <code>anylink</code></p>
        <p><strong>Client Secret:</strong> <code>secret</code></p>
        <p><strong>Redirect URI:</strong> <code>https://aaa.ai520.me:443/oidc/callback</code></p>
    </div>
    
    <div class="endpoint">
        <h3>端点</h3>
        <ul>
            <li><strong>发现端点:</strong> <a href="/.well-known/openid_configuration">/.well-known/openid_configuration</a></li>
            <li><strong>授权端点:</strong> /auth</li>
            <li><strong>令牌端点:</strong> /token</li>
            <li><strong>用户信息端点:</strong> /userinfo</li>
            <li><strong>JWKS端点:</strong> /jwks</li>
        </ul>
    </div>
    
    <div class="config">
        <h3>测试用户</h3>
        <ul>
            <li><strong>admin:</strong> 管理员用户 (组: ops, all)</li>
            <li><strong>user1:</strong> 普通用户 (组: all)</li>
            <li><strong>QQ登录:</strong> 使用QQ扫码验证QQ号真实性</li>
        </ul>
        <p>密码: 任意（此测试服务不验证密码）</p>
        <p><strong>QQ登录说明：</strong></p>
        <ul>
            <li>选择"使用QQ登录"选项</li>
            <li>使用手机QQ扫描二维码</li>
            <li>验证QQ号真实性后即可登录</li>
            <li>所有QQ用户默认分配到"all"组</li>
        </ul>
    </div>
    
    <div class="config">
        <h3>AnyLink OIDC 配置示例</h3>
        <pre>{
  "type": "oidc",
  "oidc": {
    "issuer_url": "https://aaa.ai520.me:8080",
    "client_id": "anylink",
    "client_secret": "secret",
    "redirect_uri": "https://aaa.ai520.me:443/oidc/callback",
    "scopes": "openid profile email",
    "username_claim": "preferred_username",
    "email_claim": "email",
    "groups_claim": "groups",
    "allowed_groups": "ops,all"
  }
}</pre>
    </div>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, homePage)
}

// 加载现有的SSL证书
func loadExistingCert() (tls.Certificate, error) {
	certFile := "./ssl_generate/fullchain.pem"
	keyFile := "./ssl_generate/cert.key"
	
	// 检查证书文件是否存在
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return tls.Certificate{}, fmt.Errorf("证书文件不存在: %s", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return tls.Certificate{}, fmt.Errorf("私钥文件不存在: %s", keyFile)
	}
	
	// 读取证书文件并清理BOM
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("读取证书文件失败: %v", err)
	}
	
	// 读取私钥文件
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("读取私钥文件失败: %v", err)
	}
	
	// 移除BOM字符
	certData = removeBOM(certData)
	keyData = removeBOM(keyData)
	
	// 加载证书和私钥
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("加载证书失败: %v", err)
	}
	
	return cert, nil
}

// 移除BOM字符
func removeBOM(data []byte) []byte {
	// UTF-8 BOM: 0xEF, 0xBB, 0xBF
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

func main() {
	fmt.Println("启动OIDC测试提供商...")
	
	// 加载现有证书
	cert, err := loadExistingCert()
	if err != nil {
		fmt.Printf("加载证书失败: %v\n", err)
		return
	}
	fmt.Println("成功加载SSL证书")

	// 配置TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	server := &http.Server{
		Addr:      ":8080",
		TLSConfig: tlsConfig,
	}
	
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/.well-known/openid_configuration", wellKnownHandler)
	http.HandleFunc("/auth", authHandler)
	http.HandleFunc("/token", tokenHandler)
	http.HandleFunc("/userinfo", userinfoHandler)
	http.HandleFunc("/jwks", jwksHandler)
	
	// QQ登录相关端点
	http.HandleFunc("/qq/login", qqLoginHandler)
	http.HandleFunc("/qq/qrcode/", qqQRCodeHandler)
	http.HandleFunc("/qq/status/", qqStatusHandler)
	http.HandleFunc("/qq/callback", qqCallbackHandler)
	
	fmt.Println("OIDC Provider 运行在 https://aaa.ai520.me:8080")
	fmt.Println("发现端点: https://aaa.ai520.me:8080/.well-known/openid_configuration")
	fmt.Println("注意: 使用自签名证书，浏览器会显示安全警告")
	
	if err := server.ListenAndServeTLS("", ""); err != nil {
		fmt.Printf("服务器启动失败: %v\n", err)
	}
}