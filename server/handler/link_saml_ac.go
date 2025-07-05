package handler

import (
	"fmt"
	"net/http"
	
	"github.com/bjdgyc/anylink/base"
)

// /+CSCOE+/saml_ac_login.html 端点处理 - 正版Cisco格式
func LinkSamlAcLogin(w http.ResponseWriter, r *http.Request) {
	// 生成符合正版Cisco的登录页面
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Sign In</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/+CSCOU+/saml_ac.css">
    <style>
        body {
            font-family: Arial, sans-serif;
            background-color: #f5f5f5;
            margin: 0;
            padding: 0;
        }
        .container {
            max-width: 400px;
            margin: 100px auto;
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .logo {
            text-align: center;
            margin-bottom: 30px;
        }
        .title {
            font-size: 24px;
            color: #333;
            text-align: center;
            margin-bottom: 10px;
        }
        .message {
            color: #666;
            text-align: center;
            margin-bottom: 30px;
        }
        .loading {
            text-align: center;
            margin: 30px 0;
        }
        .spinner {
            border: 3px solid #f3f3f3;
            border-top: 3px solid #3498db;
            border-radius: 50%;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 20px auto;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        .status {
            text-align: center;
            color: #666;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">
            <img src="/+CSCOU+/csco_logo.png" alt="Cisco" style="max-width: 150px;">
        </div>
        <div class="title">Authentication Complete</div>
        <div class="message">
            Your authentication has been completed successfully.
        </div>
        <div class="loading">
            <div class="spinner"></div>
            <div class="status">Configuring VPN connection...</div>
        </div>
    </div>
    <script>
        // 检查Cookie中的令牌
        function getCookie(name) {
            const value = '; ' + document.cookie;
            const parts = value.split('; ' + name + '=');
            if (parts.length === 2) return parts.pop().split(';').shift();
        }
        
        // 定期检查认证状态
        let checkCount = 0;
        const maxChecks = 10;
        
        function checkAuthStatus() {
            const token = getCookie('acSamlv2Token');
            if (token) {
                // 令牌存在，认证成功
                document.querySelector('.status').textContent = 'Authentication successful! You may close this window.';
                document.querySelector('.spinner').style.display = 'none';
                
                // 尝试关闭窗口（某些浏览器可能阻止）
                setTimeout(() => {
                    window.close();
                }, 2000);
            } else if (checkCount < maxChecks) {
                // 继续检查
                checkCount++;
                setTimeout(checkAuthStatus, 1000);
            } else {
                // 超时
                document.querySelector('.status').textContent = 'Authentication timeout. Please try again.';
                document.querySelector('.spinner').style.display = 'none';
            }
        }
        
        // 页面加载后开始检查
        window.onload = function() {
            checkAuthStatus();
        };
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(html))
}

// 处理SAML SP登录请求 - 重定向到OIDC
func LinkSamlSpLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.URL.Query().Get("ctx")
	acsamlcap := r.URL.Query().Get("acsamlcap")
	
	// 构建OIDC登录URL - 传递上下文信息
	oidcURL := fmt.Sprintf("/oidc/login?ctx=%s&acsamlcap=%s", ctx, acsamlcap)
	
	// 记录调试信息
	base.Info("LinkSamlSpLogin - Redirecting to OIDC login with ctx:", ctx)
	
	// 重定向到OIDC登录
	http.Redirect(w, r, oidcURL, http.StatusFound)
}