package handler

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/bjdgyc/anylink/base"
	"github.com/bjdgyc/anylink/dbdata"
)

// OIDCAuthInfo 认证信息
type OIDCAuthInfo struct {
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Groups         []string  `json:"groups"`
	Roles          []string  `json:"roles"`
	AccessToken    string    `json:"access_token"`
	RefreshToken   string    `json:"refresh_token"`
	IDToken        string    `json:"id_token"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	GroupName      string    `json:"group_name"`
	SessionTimeout int       `json:"session_timeout"`
	UserInfo       *dbdata.OIDCUserInfoV5 `json:"user_info"`
}

// 内存会话存储
var (
	oidcSessions     = make(map[string]*OIDCSessionV5)
	oidcAuthInfos    = make(map[string]*OIDCAuthInfo)
	sessionMutex     sync.RWMutex
	authInfoMutex    sync.RWMutex
	cleanupTicker    *time.Ticker
	cleanupStop      chan bool
)

// 初始化会话管理器
func init() {
	// 启动清理任务
	startCleanupTask()
}

// startCleanupTask 启动清理任务
func startCleanupTask() {
	cleanupTicker = time.NewTicker(5 * time.Minute)
	cleanupStop = make(chan bool, 1)
	
	go func() {
		for {
			select {
			case <-cleanupTicker.C:
				cleanupExpiredSessions()
			case <-cleanupStop:
				cleanupTicker.Stop()
				return
			}
		}
	}()
}

// stopCleanupTask 停止清理任务
func stopCleanupTask() {
	if cleanupStop != nil {
		cleanupStop <- true
	}
}

// cleanupExpiredSessions 清理过期会话
func cleanupExpiredSessions() {
	now := time.Now()
	
	// 清理OIDC会话
	sessionMutex.Lock()
	for state, session := range oidcSessions {
		if now.Sub(session.CreatedAt) > 10*time.Minute {
			delete(oidcSessions, state)
			base.Debug("清理过期OIDC会话:", state)
		}
	}
	sessionMutex.Unlock()
	
	// 清理认证信息
	authInfoMutex.Lock()
	for token, authInfo := range oidcAuthInfos {
		expireTime := time.Duration(authInfo.SessionTimeout) * time.Second
		if now.Sub(authInfo.CreatedAt) > expireTime {
			delete(oidcAuthInfos, token)
			base.Debug("清理过期认证信息:", token)
		}
	}
	authInfoMutex.Unlock()
}

// saveOIDCSession 保存OIDC会话
func saveOIDCSession(state string, session *OIDCSessionV5) error {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	oidcSessions[state] = session
	base.Debug("保存OIDC会话:", state)
	return nil
}

// getOIDCSession 获取OIDC会话
func getOIDCSession(state string) (*OIDCSessionV5, error) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	
	session, exists := oidcSessions[state]
	if !exists {
		return nil, fmt.Errorf("会话不存在或已过期")
	}
	
	// 检查会话是否过期
	if time.Since(session.CreatedAt) > 10*time.Minute {
		return nil, fmt.Errorf("会话已过期")
	}
	
	return session, nil
}

// deleteOIDCSession 删除OIDC会话
func deleteOIDCSession(state string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	delete(oidcSessions, state)
	base.Debug("删除OIDC会话:", state)
}

// saveOIDCAuthInfo 保存认证信息
func saveOIDCAuthInfo(token, username string, userInfo *dbdata.OIDCUserInfoV5, tokens *dbdata.OIDCTokensV5, groupName string) error {
	authInfoMutex.Lock()
	defer authInfoMutex.Unlock()
	
	// 获取组配置以获取会话超时时间
	group := dbdata.GetGroup(groupName)
	if group == nil {
		return fmt.Errorf("获取组配置失败: %s", groupName)
	}
	
	// 解析认证配置
	var authConfig dbdata.AuthOidcV5
	if err := parseAuthConfig(group.Auth, "oidc_v5", &authConfig); err != nil {
		return fmt.Errorf("解析认证配置失败: %v", err)
	}
	
	authInfo := &OIDCAuthInfo{
		Username:       username,
		Email:          userInfo.Email,
		Groups:         userInfo.Groups,
		Roles:          userInfo.Roles,
		AccessToken:    tokens.AccessToken,
		RefreshToken:   tokens.RefreshToken,
		IDToken:        tokens.IDToken,
		ExpiresAt:      tokens.ExpiresAt,
		CreatedAt:      time.Now(),
		GroupName:      groupName,
		SessionTimeout: authConfig.SessionTimeout,
		UserInfo:       userInfo,
	}
	
	oidcAuthInfos[token] = authInfo
	base.Debug("保存认证信息:", token, "用户:", username)
	return nil
}

// getOIDCAuthInfo 获取认证信息
func getOIDCAuthInfo(token string) (*OIDCAuthInfo, error) {
	authInfoMutex.RLock()
	defer authInfoMutex.RUnlock()
	
	authInfo, exists := oidcAuthInfos[token]
	if !exists {
		return nil, fmt.Errorf("认证信息不存在或已过期")
	}
	
	// 检查是否过期
	expireTime := time.Duration(authInfo.SessionTimeout) * time.Second
	if time.Since(authInfo.CreatedAt) > expireTime {
		return nil, fmt.Errorf("认证信息已过期")
	}
	
	return authInfo, nil
}

// deleteOIDCAuthInfo 删除认证信息
func deleteOIDCAuthInfo(token string) {
	authInfoMutex.Lock()
	defer authInfoMutex.Unlock()
	
	delete(oidcAuthInfos, token)
	base.Debug("删除认证信息:", token)
}

// generateRandomString 生成随机字符串
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

// GetOIDCSessionStats 获取会话统计信息
func GetOIDCSessionStats() map[string]interface{} {
	sessionMutex.RLock()
	authInfoMutex.RLock()
	defer sessionMutex.RUnlock()
	defer authInfoMutex.RUnlock()
	
	stats := map[string]interface{}{
		"active_sessions":    len(oidcSessions),
		"active_auth_infos":  len(oidcAuthInfos),
		"cleanup_enabled":    cleanupTicker != nil,
		"last_cleanup":       time.Now().Format("2006-01-02 15:04:05"),
	}
	
	return stats
}

// ValidateOIDCTokenForVPN 验证OIDC令牌用于VPN登录
func ValidateOIDCTokenForVPN(token string, groupName string) (*OIDCAuthInfo, error) {
	authInfo, err := getOIDCAuthInfo(token)
	if err != nil {
		return nil, err
	}
	
	// 验证组名
	if authInfo.GroupName != groupName {
		return nil, fmt.Errorf("组名不匹配")
	}
	
	// 检查令牌是否过期
	if time.Since(authInfo.CreatedAt) > time.Duration(authInfo.SessionTimeout)*time.Second {
		deleteOIDCAuthInfo(token)
		return nil, fmt.Errorf("令牌已过期")
	}
	
	return authInfo, nil
}

// RefreshOIDCAuthInfo 刷新认证信息
func RefreshOIDCAuthInfo(token string, newTokens *dbdata.OIDCTokensV5, newUserInfo *dbdata.OIDCUserInfoV5) error {
	authInfoMutex.Lock()
	defer authInfoMutex.Unlock()
	
	authInfo, exists := oidcAuthInfos[token]
	if !exists {
		return fmt.Errorf("认证信息不存在")
	}
	
	// 更新令牌信息
	authInfo.AccessToken = newTokens.AccessToken
	authInfo.RefreshToken = newTokens.RefreshToken
	authInfo.IDToken = newTokens.IDToken
	authInfo.ExpiresAt = newTokens.ExpiresAt
	authInfo.UserInfo = newUserInfo
	
	// 更新用户信息
	if newUserInfo != nil {
		authInfo.Email = newUserInfo.Email
		authInfo.Groups = newUserInfo.Groups
		authInfo.Roles = newUserInfo.Roles
	}
	
	base.Debug("刷新认证信息:", token)
	return nil
}

// GetAllActiveOIDCUsers 获取所有活跃的OIDC用户
func GetAllActiveOIDCUsers() []map[string]interface{} {
	authInfoMutex.RLock()
	defer authInfoMutex.RUnlock()
	
	var users []map[string]interface{}
	now := time.Now()
	
	for token, authInfo := range oidcAuthInfos {
		// 检查是否过期
		expireTime := time.Duration(authInfo.SessionTimeout) * time.Second
		if now.Sub(authInfo.CreatedAt) > expireTime {
			continue
		}
		
		user := map[string]interface{}{
			"token":      token,
			"username":   authInfo.Username,
			"email":      authInfo.Email,
			"groups":     authInfo.Groups,
			"roles":      authInfo.Roles,
			"group_name": authInfo.GroupName,
			"created_at": authInfo.CreatedAt,
			"expires_at": authInfo.CreatedAt.Add(expireTime),
		}
		
		users = append(users, user)
	}
	
	return users
}

// CleanupOIDCUserSessions 清理特定用户的所有会话
func CleanupOIDCUserSessions(username string) int {
	authInfoMutex.Lock()
	defer authInfoMutex.Unlock()
	
	count := 0
	for token, authInfo := range oidcAuthInfos {
		if authInfo.Username == username {
			delete(oidcAuthInfos, token)
			count++
		}
	}
	
	base.Info("清理用户会话:", username, "数量:", count)
	return count
}

// GetOIDCUserSessions 获取特定用户的会话
func GetOIDCUserSessions(username string) []map[string]interface{} {
	authInfoMutex.RLock()
	defer authInfoMutex.RUnlock()
	
	var sessions []map[string]interface{}
	now := time.Now()
	
	for token, authInfo := range oidcAuthInfos {
		if authInfo.Username == username {
			// 检查是否过期
			expireTime := time.Duration(authInfo.SessionTimeout) * time.Second
			if now.Sub(authInfo.CreatedAt) > expireTime {
				continue
			}
			
			session := map[string]interface{}{
				"token":      token,
				"created_at": authInfo.CreatedAt,
				"expires_at": authInfo.CreatedAt.Add(expireTime),
				"group_name": authInfo.GroupName,
			}
			
			sessions = append(sessions, session)
		}
	}
	
	return sessions
}