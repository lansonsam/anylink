package dbdata

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"github.com/bjdgyc/anylink/base"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	authRegistry["oidc_v5"] = reflect.TypeOf(AuthOidcV5{})
}

// AuthOidcV5 标准化OIDC认证配置
type AuthOidcV5 struct {
	// 基础配置
	IssuerURL       string `json:"issuer_url"`        // OIDC提供者URL
	ClientID        string `json:"client_id"`         // 客户端ID
	ClientSecret    string `json:"client_secret"`     // 客户端密钥
	RedirectURI     string `json:"redirect_uri"`      // 回调URL
	
	// 高级配置
	Scopes          []string `json:"scopes"`           // 权限范围，默认["openid", "profile", "email"]
	ResponseType    string   `json:"response_type"`    // 响应类型，默认"code"
	ResponseMode    string   `json:"response_mode"`    // 响应模式，可选"query", "fragment", "form_post"
	
	// 用户映射配置
	UsernameClaim   string `json:"username_claim"`    // 用户名字段映射
	EmailClaim      string `json:"email_claim"`       // 邮箱字段映射
	GroupsClaim     string `json:"groups_claim"`      // 用户组字段映射
	RolesClaim      string `json:"roles_claim"`       // 角色字段映射
	
	// 授权配置
	AllowedGroups   []string `json:"allowed_groups"`   // 允许的用户组
	AllowedRoles    []string `json:"allowed_roles"`    // 允许的角色
	RequiredClaims  map[string]string `json:"required_claims"` // 必需的声明
	
	// 会话配置
	SessionTimeout  int    `json:"session_timeout"`   // 会话超时时间(秒)，默认3600
	RefreshToken    bool   `json:"refresh_token"`     // 是否启用刷新令牌
	
	// 安全配置
	SkipTLSVerify   bool   `json:"skip_tls_verify"`   // 跳过TLS验证(仅用于测试)
	AllowedAudiences []string `json:"allowed_audiences"` // 允许的audience
	ClockSkew       int    `json:"clock_skew"`        // 时钟偏差容忍度(秒)，默认300
	
	// 内部状态
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// OIDCClientV5 标准化OIDC客户端
type OIDCClientV5 struct {
	config   *AuthOidcV5
	provider *oidc.Provider
	oauth2Config *oauth2.Config
	verifier *oidc.IDTokenVerifier
	ctx      context.Context
}

// OIDCUserInfoV5 标准化用户信息
type OIDCUserInfoV5 struct {
	Subject           string                 `json:"sub"`
	PreferredUsername string                 `json:"preferred_username"`
	Name              string                 `json:"name"`
	Email             string                 `json:"email"`
	EmailVerified     bool                   `json:"email_verified"`
	Groups            []string               `json:"groups"`
	Roles             []string               `json:"roles"`
	Claims            map[string]interface{} `json:"claims"`
	
	// 额外的标准声明
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Picture           string `json:"picture"`
	Locale            string `json:"locale"`
	UpdatedAt         int64  `json:"updated_at"`
}

// OIDCTokensV5 标准化令牌信息
type OIDCTokensV5 struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope"`
}

// OIDCErrorV5 标准化错误类型
type OIDCErrorV5 struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
	URI         string `json:"error_uri"`
	State       string `json:"state"`
}

func (e *OIDCErrorV5) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("OIDC Error [%s]: %s", e.Code, e.Description)
	}
	return fmt.Sprintf("OIDC Error: %s", e.Code)
}

// checkData 验证OIDC配置
func (auth AuthOidcV5) checkData(authData map[string]interface{}) error {
	authType := authData["type"].(string)
	bodyBytes, err := json.Marshal(authData[authType])
	if err != nil {
		return fmt.Errorf("OIDC配置解析失败: %v", err)
	}
	
	if err := json.Unmarshal(bodyBytes, &auth); err != nil {
		return fmt.Errorf("OIDC配置反序列化失败: %v", err)
	}
	
	// 验证必需字段
	if auth.IssuerURL == "" {
		return fmt.Errorf("OIDC Issuer URL不能为空")
	}
	if auth.ClientID == "" {
		return fmt.Errorf("OIDC Client ID不能为空")
	}
	if auth.ClientSecret == "" {
		return fmt.Errorf("OIDC Client Secret不能为空")
	}
	if auth.RedirectURI == "" {
		return fmt.Errorf("OIDC Redirect URI不能为空")
	}
	
	return nil
}

// NewOIDCClientV5 创建标准化OIDC客户端
func NewOIDCClientV5(config *AuthOidcV5) (*OIDCClientV5, error) {
	ctx := context.Background()
	
	// 设置默认值
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"openid", "profile", "email"}
	}
	if config.ResponseType == "" {
		config.ResponseType = "code"
	}
	if config.SessionTimeout == 0 {
		config.SessionTimeout = 3600
	}
	if config.ClockSkew == 0 {
		config.ClockSkew = 300
	}
	
	// 创建OIDC提供者
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("创建OIDC提供者失败: %v", err)
	}
	
	// 创建OAuth2配置
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       config.Scopes,
	}
	
	// 创建ID令牌验证器
	verifyConfig := &oidc.Config{
		ClientID:             config.ClientID,
		SupportedSigningAlgs: []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512"},
		SkipClientIDCheck:    false,
		SkipExpiryCheck:      false,
		SkipIssuerCheck:      false,
		Now:                  time.Now,
	}
	
	// 设置允许的audience
	if len(config.AllowedAudiences) > 0 {
		verifyConfig.SupportedSigningAlgs = config.AllowedAudiences
	}
	
	verifier := provider.Verifier(verifyConfig)
	
	client := &OIDCClientV5{
		config:       config,
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
		ctx:          ctx,
	}
	
	return client, nil
}

// GenerateStateAndNonce 生成state和nonce参数
func (c *OIDCClientV5) GenerateStateAndNonce() (string, string, error) {
	state, err := c.generateRandomString(32)
	if err != nil {
		return "", "", fmt.Errorf("生成state失败: %v", err)
	}
	
	nonce, err := c.generateRandomString(32)
	if err != nil {
		return "", "", fmt.Errorf("生成nonce失败: %v", err)
	}
	
	return state, nonce, nil
}

// GetAuthCodeURL 获取授权URL
func (c *OIDCClientV5) GetAuthCodeURL(state, nonce string, opts ...oauth2.AuthCodeOption) string {
	// 添加nonce参数
	options := append(opts, oauth2.SetAuthURLParam("nonce", nonce))
	
	// 添加响应模式
	if c.config.ResponseMode != "" {
		options = append(options, oauth2.SetAuthURLParam("response_mode", c.config.ResponseMode))
	}
	
	return c.oauth2Config.AuthCodeURL(state, options...)
}

// ExchangeCode 交换授权码获取令牌
func (c *OIDCClientV5) ExchangeCode(code, state string) (*OIDCTokensV5, error) {
	// 交换授权码
	oauth2Token, err := c.oauth2Config.Exchange(c.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("交换授权码失败: %v", err)
	}
	
	// 提取ID令牌
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("未找到ID令牌")
	}
	
	// 验证ID令牌
	_, err = c.verifier.Verify(c.ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("ID令牌验证失败: %v", err)
	}
	
	tokens := &OIDCTokensV5{
		AccessToken:  oauth2Token.AccessToken,
		TokenType:    oauth2Token.TokenType,
		RefreshToken: oauth2Token.RefreshToken,
		IDToken:      rawIDToken,
		ExpiresAt:    oauth2Token.Expiry,
		Scope:        strings.Join(c.config.Scopes, " "),
	}
	
	return tokens, nil
}

// GetUserInfo 获取用户信息
func (c *OIDCClientV5) GetUserInfo(tokens *OIDCTokensV5) (*OIDCUserInfoV5, error) {
	// 解析ID令牌
	idToken, err := c.verifier.Verify(c.ctx, tokens.IDToken)
	if err != nil {
		return nil, fmt.Errorf("验证ID令牌失败: %v", err)
	}
	
	// 提取声明
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("解析ID令牌声明失败: %v", err)
	}
	
	// 创建OAuth2令牌对象用于获取用户信息
	oauth2Token := &oauth2.Token{
		AccessToken: tokens.AccessToken,
		TokenType:   tokens.TokenType,
		Expiry:      tokens.ExpiresAt,
	}
	
	// 获取用户信息端点数据
	userInfo, err := c.provider.UserInfo(c.ctx, oauth2.StaticTokenSource(oauth2Token))
	if err != nil {
		base.Warn("获取用户信息端点失败，使用ID令牌声明: %v", err)
		// 如果用户信息端点失败，仅使用ID令牌声明
	} else {
		// 合并用户信息端点的声明到ID令牌声明中
		var userInfoClaims map[string]interface{}
		if err := userInfo.Claims(&userInfoClaims); err == nil {
			for k, v := range userInfoClaims {
				claims[k] = v
			}
		}
	}
	
	// 构建用户信息
	userInfoV5 := &OIDCUserInfoV5{
		Claims: claims,
	}
	
	// 映射标准字段
	if sub, ok := claims["sub"].(string); ok {
		userInfoV5.Subject = sub
	}
	if preferredUsername, ok := claims["preferred_username"].(string); ok {
		userInfoV5.PreferredUsername = preferredUsername
	}
	if name, ok := claims["name"].(string); ok {
		userInfoV5.Name = name
	}
	if email, ok := claims["email"].(string); ok {
		userInfoV5.Email = email
	}
	if emailVerified, ok := claims["email_verified"].(bool); ok {
		userInfoV5.EmailVerified = emailVerified
	}
	if givenName, ok := claims["given_name"].(string); ok {
		userInfoV5.GivenName = givenName
	}
	if familyName, ok := claims["family_name"].(string); ok {
		userInfoV5.FamilyName = familyName
	}
	if picture, ok := claims["picture"].(string); ok {
		userInfoV5.Picture = picture
	}
	if locale, ok := claims["locale"].(string); ok {
		userInfoV5.Locale = locale
	}
	if updatedAt, ok := claims["updated_at"].(float64); ok {
		userInfoV5.UpdatedAt = int64(updatedAt)
	}
	
	// 提取用户组信息
	userInfoV5.Groups = c.extractStringArrayFromClaim(claims, c.config.GroupsClaim, "groups")
	
	// 提取角色信息
	userInfoV5.Roles = c.extractStringArrayFromClaim(claims, c.config.RolesClaim, "roles")
	
	return userInfoV5, nil
}

// ValidateUser 验证用户权限
func (c *OIDCClientV5) ValidateUser(userInfo *OIDCUserInfoV5) error {
	// 检查必需声明
	if err := c.validateRequiredClaims(userInfo.Claims); err != nil {
		return err
	}
	
	// 检查用户组权限
	if len(c.config.AllowedGroups) > 0 {
		if !c.hasAnyOfValues(userInfo.Groups, c.config.AllowedGroups) {
			return fmt.Errorf("用户不在允许的组中。用户组: %v, 允许的组: %v", 
				userInfo.Groups, c.config.AllowedGroups)
		}
	}
	
	// 检查角色权限
	if len(c.config.AllowedRoles) > 0 {
		if !c.hasAnyOfValues(userInfo.Roles, c.config.AllowedRoles) {
			return fmt.Errorf("用户没有所需的角色。用户角色: %v, 允许的角色: %v", 
				userInfo.Roles, c.config.AllowedRoles)
		}
	}
	
	return nil
}

// GetUsername 获取用户名
func (c *OIDCClientV5) GetUsername(userInfo *OIDCUserInfoV5) string {
	// 优先使用配置的用户名声明
	if c.config.UsernameClaim != "" {
		if username, ok := userInfo.Claims[c.config.UsernameClaim].(string); ok && username != "" {
			return username
		}
	}
	
	// 回退到标准字段
	if userInfo.PreferredUsername != "" {
		return userInfo.PreferredUsername
	}
	if userInfo.Email != "" {
		return userInfo.Email
	}
	
	return userInfo.Subject
}

// RefreshTokens 刷新令牌
func (c *OIDCClientV5) RefreshTokens(refreshToken string) (*OIDCTokensV5, error) {
	if !c.config.RefreshToken || refreshToken == "" {
		return nil, fmt.Errorf("刷新令牌功能未启用或刷新令牌为空")
	}
	
	tokenSource := c.oauth2Config.TokenSource(c.ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	})
	
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("刷新令牌失败: %v", err)
	}
	
	rawIDToken, ok := newToken.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("刷新后未找到ID令牌")
	}
	
	tokens := &OIDCTokensV5{
		AccessToken:  newToken.AccessToken,
		TokenType:    newToken.TokenType,
		RefreshToken: newToken.RefreshToken,
		IDToken:      rawIDToken,
		ExpiresAt:    newToken.Expiry,
		Scope:        strings.Join(c.config.Scopes, " "),
	}
	
	return tokens, nil
}

// ValidateIDToken 验证ID令牌
func (c *OIDCClientV5) ValidateIDToken(rawIDToken string) (*jwt.Token, error) {
	// 使用go-oidc库验证
	idToken, err := c.verifier.Verify(c.ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("OIDC验证失败: %v", err)
	}
	
	// 提取声明用于额外验证
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("解析ID令牌声明失败: %v", err)
	}
	
	// 创建JWT令牌对象（为了兼容现有代码）
	token := &jwt.Token{
		Raw:    rawIDToken,
		Method: jwt.SigningMethodRS256, // 默认方法，实际由go-oidc验证
		Header: map[string]interface{}{
			"typ": "JWT",
			"alg": "RS256",
		},
		Claims: jwt.MapClaims(claims),
		Valid:  true,
	}
	
	return token, nil
}

// 工具函数

func (c *OIDCClientV5) generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func (c *OIDCClientV5) extractStringArrayFromClaim(claims map[string]interface{}, configField, defaultField string) []string {
	var result []string
	
	// 首先尝试使用配置的字段
	field := defaultField
	if configField != "" {
		field = configField
	}
	
	value, ok := claims[field]
	if !ok {
		return result
	}
	
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
	case []string:
		result = v
	case string:
		result = []string{v}
	}
	
	return result
}

func (c *OIDCClientV5) validateRequiredClaims(claims map[string]interface{}) error {
	for key, expectedValue := range c.config.RequiredClaims {
		actualValue, ok := claims[key]
		if !ok {
			return fmt.Errorf("缺少必需的声明: %s", key)
		}
		
		actualStr := fmt.Sprintf("%v", actualValue)
		if actualStr != expectedValue {
			return fmt.Errorf("声明 %s 的值不匹配。期望: %s, 实际: %s", key, expectedValue, actualStr)
		}
	}
	
	return nil
}

func (c *OIDCClientV5) hasAnyOfValues(userValues, allowedValues []string) bool {
	for _, userValue := range userValues {
		for _, allowedValue := range allowedValues {
			if strings.TrimSpace(userValue) == strings.TrimSpace(allowedValue) {
				return true
			}
		}
	}
	return false
}

// checkUser 实现认证接口
func (auth AuthOidcV5) checkUser(name, pwd string, g *Group, ext map[string]interface{}) error {
	// 检查是否为OIDC令牌
	if strings.HasPrefix(pwd, "oidc_v5_token_") {
		// 令牌验证在handler层处理
		return nil
	}
	
	// 普通密码登录触发OIDC流程
	return fmt.Errorf("oidc_v5:required")
}