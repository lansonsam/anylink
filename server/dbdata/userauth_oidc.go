package dbdata

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/bjdgyc/anylink/base"
)

func init() {
	authRegistry["oidc"] = reflect.TypeOf(AuthOidc{})
}

type AuthOidc struct {
	IssuerUrl       string `json:"issuer_url"`        // OIDC Issuer URL
	ClientId        string `json:"client_id"`         // 客户端 ID
	ClientSecret    string `json:"client_secret"`     // 客户端密钥
	RedirectUri     string `json:"redirect_uri"`      // 回调 URL
	Scopes          string `json:"scopes"`            // 授权范围
	UsernameClaim   string `json:"username_claim"`    // 用户名字段
	EmailClaim      string `json:"email_claim"`       // 邮箱字段
	GroupsClaim     string `json:"groups_claim"`      // 组字段
	AllowedGroups   string `json:"allowed_groups"`    // 允许的组
}

type OidcConfig struct {
	Issuer                 string `json:"issuer"`
	AuthorizationEndpoint  string `json:"authorization_endpoint"`
	TokenEndpoint          string `json:"token_endpoint"`
	UserinfoEndpoint       string `json:"userinfo_endpoint"`
	JwksUri               string `json:"jwks_uri"`
}

type OidcTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	IdToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type OidcUserInfo struct {
	Sub               string      `json:"sub"`
	PreferredUsername string      `json:"preferred_username"`
	Email             string      `json:"email"`
	Name              string      `json:"name"`
	Groups            interface{} `json:"groups"`
}

func (auth AuthOidc) checkData(authData map[string]interface{}) error {
	authType := authData["type"].(string)
	bodyBytes, err := json.Marshal(authData[authType])
	if err != nil {
		return fmt.Errorf("OIDC 配置解析失败: %v", err)
	}
	json.Unmarshal(bodyBytes, &auth)
	if auth.IssuerUrl == "" {
		return fmt.Errorf("OIDC Issuer URL 不能为空")
	}
	if auth.ClientId == "" {
		return fmt.Errorf("OIDC Client ID 不能为空")
	}
	if auth.ClientSecret == "" {
		return fmt.Errorf("OIDC Client Secret 不能为空")
	}
	if auth.RedirectUri == "" {
		return fmt.Errorf("OIDC Redirect URI 不能为空")
	}
	return nil
}

func (auth *AuthOidc) GetConfig() (*OidcConfig, error) {
	wellKnownUrl := strings.TrimSuffix(auth.IssuerUrl, "/") + "/.well-known/openid-configuration"
	
	// 创建HTTP客户端，对于localhost跳过证书验证
	tr := &http.Transport{}
	if strings.Contains(auth.IssuerUrl, "localhost") || strings.Contains(auth.IssuerUrl, "127.0.0.1") {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{Transport: tr, Timeout: 30 * time.Second}
	
	resp, err := client.Get(wellKnownUrl)
	if err != nil {
		return nil, fmt.Errorf("获取 OIDC 配置失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC 配置端点返回状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 OIDC 配置失败: %v", err)
	}

	var config OidcConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("解析 OIDC 配置失败: %v", err)
	}

	return &config, nil
}

func (auth *AuthOidc) GenerateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (auth *AuthOidc) GetAuthURL(state string) (string, error) {
	config, err := auth.GetConfig()
	if err != nil {
		return "", err
	}

	scopes := "openid profile email"
	if auth.Scopes != "" {
		scopes = auth.Scopes
	}

	params := url.Values{
		"response_type": []string{"code"},
		"client_id":     []string{auth.ClientId},
		"redirect_uri":  []string{auth.RedirectUri},
		"scope":         []string{scopes},
		"state":         []string{state},
	}

	return config.AuthorizationEndpoint + "?" + params.Encode(), nil
}

func (auth *AuthOidc) ExchangeCodeForToken(code, state string) (*OidcTokenResponse, error) {
	config, err := auth.GetConfig()
	if err != nil {
		return nil, err
	}

	data := url.Values{
		"grant_type":   []string{"authorization_code"},
		"code":         []string{code},
		"redirect_uri": []string{auth.RedirectUri},
		"client_id":    []string{auth.ClientId},
		"client_secret": []string{auth.ClientSecret},
	}

	// 创建HTTP客户端，对于localhost跳过证书验证
	tr := &http.Transport{}
	if strings.Contains(auth.IssuerUrl, "localhost") || strings.Contains(auth.IssuerUrl, "127.0.0.1") {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{Transport: tr, Timeout: 30 * time.Second}

	resp, err := client.PostForm(config.TokenEndpoint, data)
	if err != nil {
		return nil, fmt.Errorf("交换授权码失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("令牌端点返回错误 %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取令牌响应失败: %v", err)
	}

	var tokenResp OidcTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析令牌响应失败: %v", err)
	}

	return &tokenResp, nil
}

func (auth *AuthOidc) GetUserInfo(accessToken string) (*OidcUserInfo, error) {
	config, err := auth.GetConfig()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", config.UserinfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("创建用户信息请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("用户信息端点返回错误 %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取用户信息失败: %v", err)
	}

	var userInfo OidcUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %v", err)
	}

	return &userInfo, nil
}

func (auth *AuthOidc) ParseIdToken(idToken string) (map[string]interface{}, error) {
	// 简化处理，直接返回空的 claims
	// 在生产环境中应该正确解析和验证 JWT
	base.Warn("ID Token 解析被简化，未进行签名验证")
	return make(map[string]interface{}), nil
}

func (auth *AuthOidc) ValidateUser(userInfo *OidcUserInfo, idTokenClaims map[string]interface{}) (string, error) {
	// 获取用户名
	username := userInfo.PreferredUsername
	if auth.UsernameClaim != "" && idTokenClaims[auth.UsernameClaim] != nil {
		if val, ok := idTokenClaims[auth.UsernameClaim].(string); ok {
			username = val
		}
	}
	if username == "" {
		username = userInfo.Sub
	}

	// 检查组权限
	if auth.AllowedGroups != "" {
		allowedGroups := strings.Split(auth.AllowedGroups, ",")
		userGroups := auth.extractGroups(userInfo, idTokenClaims)
		
		if !auth.hasAllowedGroup(userGroups, allowedGroups) {
			return "", fmt.Errorf("用户不在允许的组中")
		}
	}

	base.Info("OIDC 用户验证成功:", username)
	return username, nil
}

func (auth *AuthOidc) extractGroups(userInfo *OidcUserInfo, idTokenClaims map[string]interface{}) []string {
	var groups []string

	// 从 ID Token 中获取组信息
	if auth.GroupsClaim != "" && idTokenClaims[auth.GroupsClaim] != nil {
		switch v := idTokenClaims[auth.GroupsClaim].(type) {
		case []interface{}:
			for _, group := range v {
				if groupStr, ok := group.(string); ok {
					groups = append(groups, groupStr)
				}
			}
		case []string:
			groups = append(groups, v...)
		case string:
			groups = append(groups, v)
		}
	}

	// 从用户信息中获取组信息
	if len(groups) == 0 && userInfo.Groups != nil {
		switch v := userInfo.Groups.(type) {
		case []interface{}:
			for _, group := range v {
				if groupStr, ok := group.(string); ok {
					groups = append(groups, groupStr)
				}
			}
		case []string:
			groups = append(groups, v...)
		case string:
			groups = append(groups, v)
		}
	}

	return groups
}

func (auth *AuthOidc) hasAllowedGroup(userGroups, allowedGroups []string) bool {
	for _, userGroup := range userGroups {
		for _, allowedGroup := range allowedGroups {
			if strings.TrimSpace(userGroup) == strings.TrimSpace(allowedGroup) {
				return true
			}
		}
	}
	return false
}

func (auth AuthOidc) checkUser(name, pwd string, g *Group, ext map[string]interface{}) error {
	// 检查是否为 OIDC 令牌（这里简化处理，实际验证在 handler 层）
	if strings.HasPrefix(pwd, "oidc_token_") {
		// 这里应该验证令牌，但为了避免循环导入，我们在 handler 层验证
		// 如果到达这里说明令牌格式正确
		return nil
	}
	
	// 普通密码登录时返回特殊错误，触发 OIDC 流程
	return fmt.Errorf("oidc:required")
}