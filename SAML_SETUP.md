# AnyLink SAML 网页授权登录配置指南

## 功能说明

此功能允许用户通过网页浏览器进行 SAML 2.0 单点登录认证，支持与企业 IdP（身份提供商）集成，如 Azure AD、Okta、OneLogin 等。

## 工作流程

1. 用户在 AnyConnect 客户端选择配置了 SAML 认证的组
2. 客户端自动打开浏览器，跳转到 IdP 登录页面
3. 用户在 IdP 完成身份验证
4. 认证成功后，浏览器跳转回 AnyLink 服务器
5. 服务器验证 SAML 响应并创建 VPN 会话
6. 客户端自动连接 VPN

## 配置步骤

### 1. 在 IdP 配置 SAML 应用

以 Azure AD 为例：

1. 登录 Azure Portal
2. 进入 Azure Active Directory > 企业应用程序
3. 创建新应用程序（非库应用程序）
4. 配置 SAML 单一登录：
   - 标识符（实体 ID）：`https://your-anylink-server.com`
   - 回复 URL（断言使用者服务 URL）：`https://your-anylink-server.com/saml/acs`
   - 登录 URL：`https://your-anylink-server.com/saml/login`
5. 下载联合元数据 XML 或记录以下信息：
   - IdP SSO URL
   - IdP Entity ID  
   - IdP 证书

### 2. 在 AnyLink 配置 SAML 认证

编辑组的认证配置，将 `auth` 字段设置为：

```json
{
  "type": "saml",
  "idp_sso_url": "https://login.microsoftonline.com/xxx/saml2",
  "idp_entity_id": "https://sts.windows.net/xxx/",
  "sp_entity_id": "https://your-anylink-server.com",
  "sp_acs_url": "https://your-anylink-server.com/saml/acs",
  "idp_certificate": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
  "attribute_mapping": {
    "username": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
    "email": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
    "groups": "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"
  }
}
```

### 3. 配置参数说明

- `type`: 必须为 "saml"
- `idp_sso_url`: IdP 的 SAML SSO 端点 URL
- `idp_entity_id`: IdP 的实体 ID
- `sp_entity_id`: 服务提供商（AnyLink）的实体 ID，通常是服务器 URL
- `sp_acs_url`: 断言使用者服务 URL，用于接收 SAML 响应
- `idp_certificate`: IdP 的签名证书（PEM 格式）
- `attribute_mapping`: SAML 属性映射
  - `username`: 用户名属性的 claim URI
  - `email`: 邮箱属性的 claim URI（可选）
  - `groups`: 用户组属性的 claim URI（可选）

### 4. 客户端使用

1. 在 AnyConnect 客户端添加服务器地址
2. 在组选择界面选择配置了 SAML 的组
3. 点击连接后，会自动打开浏览器
4. 在浏览器中完成 IdP 身份验证
5. 认证成功后，浏览器会显示成功页面
6. 返回客户端，连接会自动建立

## 常见 IdP 配置示例

### Azure AD
```json
{
  "type": "saml",
  "idp_sso_url": "https://login.microsoftonline.com/{tenant-id}/saml2",
  "idp_entity_id": "https://sts.windows.net/{tenant-id}/",
  "attribute_mapping": {
    "username": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
    "email": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
  }
}
```

### Okta
```json
{
  "type": "saml",
  "idp_sso_url": "https://{okta-domain}/app/{app-id}/sso/saml",
  "idp_entity_id": "http://www.okta.com/{app-id}",
  "attribute_mapping": {
    "username": "username",
    "email": "email"
  }
}
```

### OneLogin
```json
{
  "type": "saml",
  "idp_sso_url": "https://{subdomain}.onelogin.com/trust/saml2/http-post/sso/{app-id}",
  "idp_entity_id": "https://{subdomain}.onelogin.com/saml/metadata/{app-id}",
  "attribute_mapping": {
    "username": "User.Username",
    "email": "User.email"
  }
}
```

## 故障排查

1. **浏览器未自动打开**
   - 确保客户端版本支持 SSO
   - 检查组配置中的认证类型是否为 "saml"

2. **认证后无法连接**
   - 检查 SAML 响应中的用户名属性映射是否正确
   - 查看服务器日志中的 SAML 验证错误

3. **证书验证失败**
   - 确保 IdP 证书格式正确（PEM 格式）
   - 检查证书是否过期

## 安全建议

1. 始终使用 HTTPS 协议
2. 定期更新 IdP 证书
3. 限制 SAML 断言的有效期
4. 启用 SAML 响应签名验证
5. 使用强密码策略和多因素认证

## API 端点

- GET `/saml/login?group={group_name}` - 发起 SAML 登录
- POST `/saml/acs` - SAML 断言接收端点
- GET `/saml/token?token={temp_token}` - 获取会话令牌