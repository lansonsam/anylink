# 牛子公益API - 本地验证+远程绑定部署指南

## 架构说明

### 🏠 本地验证端
- **功能**：QQ号真实性验证（二维码登录）
- **文件**：`local_qq_verifier.py`
- **依赖**：`qq_group_api.py`、`requests`

### 🌐 远程服务端  
- **功能**：接收验证凭证，处理卡密绑定
- **文件**：`remote_api_server.py`
- **数据库**：`remote_qq_bindings.db`

## 部署方案

### 方案一：本地验证 + 远程API

```
[本地客户端] --QQ验证--> [本地验证器] --验证凭证--> [远程API服务器]
     ↓                       ↓                        ↓
  用户输入QQ号            生成二维码               存储绑定关系
  用户输入卡密            扫码验证QQ               返回绑定结果
```

## 详细部署步骤

### 1. 远程服务器部署

#### 安装依赖
```bash
pip install flask requests
```

#### 启动远程API服务器
```bash
python remote_api_server.py
```

#### 配置外网访问（可选）
```bash
# 使用nginx反向代理
# 或使用gunicorn
pip install gunicorn
gunicorn -w 4 -b 0.0.0.0:5000 remote_api_server:app
```

### 2. 本地验证端部署

#### 安装依赖
```bash
pip install requests
# 确保有 qq_group_api.py 文件
```

#### 配置远程服务器地址
编辑 `local_qq_verifier.py`：
```python
REMOTE_API_URL = "http://your-domain.com:5000"  # 修改为实际地址
```

#### 运行本地验证器
```bash
python local_qq_verifier.py
```

## 使用流程

### 完整操作流程

1. **启动远程服务器**
```bash
python remote_api_server.py
```

2. **运行本地验证器**
```bash
python local_qq_verifier.py
```

3. **输入信息**
```
🔢 请输入QQ号: 123456789
💳 请输入卡密: snNWnbJ3HY5NU87v
🌐 远程API地址 (默认: http://your-server.com:5000): 
```

4. **QQ验证过程**
```
🚀 开始本地QQ验证: 123456789
📱 正在生成二维码...
✅ 二维码已生成
📱 请使用手机QQ扫描 qq_login_qr.png 文件
⏳ 等待QQ登录验证...
✅ QQ登录成功，验证通过！
```

5. **远程绑定过程**
```
🔑 生成验证凭证: a1b2c3d4e5f6789012...
🌐 正在连接远程API绑定卡密...
✅ 远程绑定成功！
👤 QQ号: 123456789
💳 卡密: snNWnbJ3HY5NU87v
🕒 绑定时间: 2023-12-21T10:30:45.123456
```

## API接口文档

### 远程服务器接口

#### 1. 首页信息
```http
GET /
```

#### 2. 验证凭证绑定
```http
POST /api/bind_with_proof
Content-Type: application/json

{
  "qq_number": "123456789",
  "card_key": "snNWnbJ3HY5NU87v",
  "verification_proof": "a1b2c3d4e5f6...",
  "verified_time": "2023-12-21T10:30:45.123456",
  "verifier_type": "local_qq_verifier"
}
```

#### 3. 查询绑定状态
```http
POST /api/query_binding
Content-Type: application/json

{
  "qq_number": "123456789"
}
```

#### 4. 获取统计信息
```http
GET /api/stats
```

## 安全考虑

### 验证凭证机制
- **生成算法**：`SHA256(QQ号-时间戳-随机数-LOCAL_VERIFIED)`
- **验证检查**：64位十六进制字符串格式验证
- **防重放**：包含时间戳，可添加过期检查

### 增强安全性（可选）
```python
# 在 remote_api_server.py 中添加
def validate_verification_proof_enhanced(qq_number: str, proof: str, verified_time: str) -> bool:
    # 检查验证时间是否在合理范围内（如10分钟内）
    from datetime import datetime, timedelta
    
    try:
        verify_time = datetime.fromisoformat(verified_time.replace('Z', '+00:00'))
        now = datetime.now()
        
        if now - verify_time > timedelta(minutes=10):
            return False  # 验证凭证已过期
            
        return True
    except:
        return False
```

## 配置参数

### 本地验证器配置
```python
# local_qq_verifier.py
REMOTE_API_URL = "http://your-server.com:5000"
TIMEOUT_SECONDS = 30
MAX_RETRY_ATTEMPTS = 3
```

### 远程服务器配置
```python
# remote_api_server.py
API_NAME = "牛子公益API - 远程服务器"
DATABASE_FILE = "remote_qq_bindings.db"
VERIFICATION_SECRET = "your_secret_key_here"
HOST = "0.0.0.0"  # 允许外部访问
PORT = 5000
```

## 数据库结构

### qq_bindings 表
| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| qq_number | TEXT | QQ号（唯一） |
| card_key | TEXT | 卡密 |
| verification_proof | TEXT | 验证凭证 |
| verified_time | TEXT | 验证时间 |
| verifier_type | TEXT | 验证器类型 |
| bind_time | TIMESTAMP | 绑定时间 |
| last_update | TIMESTAMP | 最后更新时间 |

### verification_logs 表
| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| qq_number | TEXT | QQ号 |
| action | TEXT | 操作类型 |
| result | TEXT | 操作结果 |
| client_ip | TEXT | 客户端IP |
| timestamp | TIMESTAMP | 操作时间 |

## 监控和日志

### 查看绑定统计
```bash
curl http://your-server.com:5000/api/stats
```

### 查看服务器日志
```bash
tail -f server.log  # 如果配置了日志文件
```

### 查询特定QQ绑定
```bash
curl -X POST http://your-server.com:5000/api/query_binding \
  -H "Content-Type: application/json" \
  -d '{"qq_number": "123456789"}'
```

## 故障排除

### 常见问题

1. **连接远程服务器失败**
```
❌ 无法连接到远程服务器
```
**解决**：检查网络连接和服务器地址

2. **验证凭证无效**
```
❌ 远程绑定失败: 验证凭证无效
```
**解决**：确保本地验证成功，检查凭证生成逻辑

3. **QQ验证失败**
```
❌ 本地验证失败: QQ登录失败或二维码过期
```
**解决**：重新生成二维码，及时扫描

### 调试模式

启用详细日志：
```python
import logging
logging.basicConfig(level=logging.DEBUG)
```

## 生产环境建议

1. **使用HTTPS**：配置SSL证书
2. **添加认证**：API密钥或JWT认证
3. **限流保护**：防止恶意请求
4. **数据备份**：定期备份数据库
5. **监控告警**：服务可用性监控

---

**牛子公益API**  
本地验证 + 远程绑定解决方案 