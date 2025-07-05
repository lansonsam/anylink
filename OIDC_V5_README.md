# AnyLink VPN Server v5.0 - 标准化OIDC认证

## 🚀 版本更新概览

AnyLink VPN Server v5.0 引入了全新的标准化OIDC认证系统，完全符合OpenID Connect Core 1.0规范，提供更安全、更可靠、更易于集成的身份认证解决方案。

## ✨ 主要新功能

### 🔐 标准化OIDC实现
- 基于 `github.com/coreos/go-oidc/v3/oidc` 标准库
- 完整的OpenID Connect Core 1.0规范支持
- 符合RFC 6749 (OAuth 2.0) 和 RFC 6750 (Bearer Token)

### 🛡️ 增强安全性
- ✅ 完整的JWT签名验证
- ✅ ID Token标准验证
- ✅ 防重放攻击 (nonce)
- ✅ CSRF保护 (state)
- ✅ 时钟偏差容忍度
- ✅ Audience验证

### 🔄 令牌管理
- ✅ 访问令牌和刷新令牌支持
- ✅ 自动令牌刷新机制
- ✅ 令牌过期处理
- ✅ 安全的令牌存储

### 👥 用户权限管理
- ✅ 用户组权限控制
- ✅ 角色权限管理
- ✅ 自定义声明验证
- ✅ 灵活的用户映射

### 🌐 多提供商支持
- ✅ Authing
- ✅ Keycloak  
- ✅ Azure AD
- ✅ Google
- ✅ 自定义OIDC提供商

## 📋 API端点

### OIDC V5 端点
```
GET  /oidc/v5/login      - 发起OIDC认证
GET  /oidc/v5/callback   - 处理OIDC回调
GET  /oidc/v5/token      - 验证令牌
GET  /oidc/v5/logout     - 退出登录
POST /oidc/v5/refresh    - 刷新令牌
```

### 兼容端点
```
GET  /oidc/v5/auth       - 兼容旧版认证入口
GET  /oidc/v5/verify     - 兼容旧版令牌验证
```

## 🔧 配置示例

### 基础配置
```json
{
  \"type\": \"oidc_v5\",
  \"oidc_v5\": {
    \"issuer_url\": \"https://your-oidc-provider.com\",
    \"client_id\": \"your-client-id\",
    \"client_secret\": \"your-client-secret\",
    \"redirect_uri\": \"https://your-vpn.com/oidc/v5/callback\"
  }
}
```

### 高级配置
```json
{
  \"type\": \"oidc_v5\",
  \"oidc_v5\": {
    \"issuer_url\": \"https://your-oidc-provider.com\",
    \"client_id\": \"your-client-id\",
    \"client_secret\": \"your-client-secret\",
    \"redirect_uri\": \"https://your-vpn.com/oidc/v5/callback\",
    \"scopes\": [\"openid\", \"profile\", \"email\", \"groups\"],
    \"username_claim\": \"preferred_username\",
    \"email_claim\": \"email\",
    \"groups_claim\": \"groups\",
    \"allowed_groups\": [\"vpn-users\", \"employees\"],
    \"session_timeout\": 3600,
    \"refresh_token\": true,
    \"clock_skew\": 300
  }
}
```

### Authing配置示例
```json
{
  \"type\": \"oidc_v5\",
  \"oidc_v5\": {
    \"issuer_url\": \"https://your-app.authing.cn/oidc\",
    \"client_id\": \"6867dc0d781a093d01f58595\",
    \"client_secret\": \"0a4d656d166f22d21ed2e44665e8d00b\",
    \"redirect_uri\": \"https://your-vpn.com/oidc/v5/callback\",
    \"scopes\": [\"openid\", \"username\", \"profile\", \"email\", \"phone\"],
    \"username_claim\": \"username\",
    \"session_timeout\": 7200
  }
}
```

## 🔀 迁移指南

### 从OIDC v4迁移到v5

1. **更新认证类型**
   ```json
   // 旧配置
   {\"type\": \"oidc\"}
   
   // 新配置  
   {\"type\": \"oidc_v5\"}
   ```

2. **更新端点URL**
   ```
   旧: /oidc/login -> 新: /oidc/v5/login
   旧: /oidc/callback -> 新: /oidc/v5/callback
   ```

3. **配置结构调整**
   - `scopes` 字段从字符串改为数组
   - 新增 `refresh_token`, `clock_skew` 等字段
   - 权限控制字段调整为数组格式

### 向后兼容
- v4.x配置仍然支持，不会中断现有部署
- v5提供更强的安全性和功能，建议逐步迁移

## 🧪 测试功能

运行测试脚本验证OIDC v5功能：

```bash
go run test_oidc_v5.go
```

测试包括：
- ✅ OIDC客户端基本功能
- ✅ 会话管理器
- ✅ API端点
- ✅ 配置验证

## 📚 配置参考

详细配置选项参见: `docs/oidc_v5_config_example.json`

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `issuer_url` | string | ✅ | OIDC提供者URL |
| `client_id` | string | ✅ | 客户端ID |
| `client_secret` | string | ✅ | 客户端密钥 |
| `redirect_uri` | string | ✅ | 回调URL |
| `scopes` | array | ❌ | 权限范围 |
| `username_claim` | string | ❌ | 用户名字段映射 |
| `email_claim` | string | ❌ | 邮箱字段映射 |
| `groups_claim` | string | ❌ | 用户组字段映射 |
| `allowed_groups` | array | ❌ | 允许的用户组 |
| `session_timeout` | int | ❌ | 会话超时(秒) |
| `refresh_token` | bool | ❌ | 启用刷新令牌 |
| `clock_skew` | int | ❌ | 时钟偏差容忍度(秒) |

## 🐛 故障排除

### 常见问题

1. **JWT验证失败**
   - 检查OIDC提供者的JWKS端点
   - 确认时钟同步
   - 验证audience配置

2. **权限验证失败**
   - 检查用户组声明字段
   - 确认allowed_groups配置
   - 查看用户信息端点返回数据

3. **会话过期问题**
   - 调整session_timeout设置
   - 启用refresh_token功能
   - 检查令牌有效期

### 日志调试
```bash
# 启用详细日志
export ANYLINK_LOG_LEVEL=debug
```

## 🚀 性能优化

- 内存会话管理，避免数据库I/O
- 自动清理过期会话
- JWT验证缓存
- 并发安全的会话存储

## 🛡️ 安全建议

1. **生产环境配置**
   - 禁用 `skip_tls_verify`
   - 设置合理的 `session_timeout`
   - 配置 `allowed_audiences`

2. **网络安全**
   - 使用HTTPS
   - 配置防火墙规则
   - 限制回调URL域名

3. **令牌管理**
   - 定期轮换客户端密钥
   - 监控异常认证行为
   - 实施会话清理策略

## 📈 监控指标

通过 `/oidc/v5/stats` 端点获取：
- 活跃会话数
- 认证成功/失败率
- 令牌刷新频率
- 会话清理统计

## 🤝 社区与支持

- 📖 文档: [AnyLink Wiki](https://github.com/bjdgyc/anylink/wiki)
- 🐛 问题反馈: [GitHub Issues](https://github.com/bjdgyc/anylink/issues)
- 💬 讨论: [GitHub Discussions](https://github.com/bjdgyc/anylink/discussions)

---

**AnyLink VPN Server v5.0** - 为企业提供更安全、更标准、更易集成的OIDC认证解决方案。