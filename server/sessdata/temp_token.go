package sessdata

import (
	"sync"
	"time"
)

type TempTokenData struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
	GroupName string
	Data      map[string]interface{}
}

var (
	tempTokens   = make(map[string]*TempTokenData)
	tempTokenMux sync.RWMutex
)

// SetTempToken 保存临时令牌
func SetTempToken(tempToken, sessionToken string) {
	tempTokenMux.Lock()
	defer tempTokenMux.Unlock()
	
	// 清理过期的令牌（超过5分钟）
	now := time.Now()
	for k, v := range tempTokens {
		if now.Sub(v.CreatedAt) > 5*time.Minute {
			delete(tempTokens, k)
		}
	}
	
	tempTokens[tempToken] = &TempTokenData{
		Token:     sessionToken,
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
		Data:      make(map[string]interface{}),
	}
}

// GetTempToken 获取临时令牌（不删除）
func GetTempToken(tempToken string) *TempTokenData {
	tempTokenMux.RLock()
	defer tempTokenMux.RUnlock()
	
	data, exists := tempTokens[tempToken]
	if !exists {
		return nil
	}
	
	// 检查是否过期
	if time.Now().After(data.ExpiresAt) {
		return nil
	}
	
	return data
}

// GetTempTokenString 获取并删除临时令牌（保持向后兼容）
func GetTempTokenString(tempToken string) string {
	tempTokenMux.Lock()
	defer tempTokenMux.Unlock()
	
	data, exists := tempTokens[tempToken]
	if !exists {
		return ""
	}
	
	// 检查是否过期
	if time.Now().After(data.ExpiresAt) {
		delete(tempTokens, tempToken)
		return ""
	}
	
	// 获取后立即删除
	sessionToken := data.Token
	delete(tempTokens, tempToken)
	
	return sessionToken
}

// StoreTempToken 存储完整的令牌数据
func StoreTempToken(tokenData *TempTokenData) {
	tempTokenMux.Lock()
	defer tempTokenMux.Unlock()
	
	// 清理过期的令牌
	now := time.Now()
	for k, v := range tempTokens {
		if now.After(v.ExpiresAt) {
			delete(tempTokens, k)
		}
	}
	
	tempTokens[tokenData.Token] = tokenData
}

// DeleteTempToken 删除临时令牌
func DeleteTempToken(token string) {
	tempTokenMux.Lock()
	defer tempTokenMux.Unlock()
	delete(tempTokens, token)
}

// GenerateSessionToken 生成会话令牌
func GenerateSessionToken() string {
	return generateRandomString(32)
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}