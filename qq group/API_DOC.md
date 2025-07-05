# 牛子公益API 文档 v2.0

## 概述
QQ验证令牌绑定卡密系统API - 通过QQ登录验证QQ号真实性，生成安全令牌后绑定卡密

## 核心特性
- **QQ号真实性验证**：仅验证QQ号能够成功登录，无需验证群组
- **安全令牌机制**：验证成功后生成30分钟有效期的验证令牌
- **防乱输机制**：必须通过QQ登录验证才能获得绑定权限
- **一次性令牌**：每个令牌只能使用一次，使用后自动失效

## 基础信息
- 服务地址: `http://localhost:5000`
- 数据格式: JSON
- 请求方式: POST (除首页外)
- 令牌有效期: 30分钟

## API接口

### 1. 首页/API信息
**GET** `/`
```json
{
  "api_name": "牛子公益API",
  "version": "2.0",
  "description": "QQ验证令牌绑定卡密系统API",
  "endpoints": {...},
  "flow": [
    "1. 调用 /api/qq/verify 开始验证",
    "2. 扫描二维码登录QQ",
    "3. 调用 /api/qq/status 获取验证令牌",
    "4. 使用令牌调用 /api/qq/bind 绑定卡密"
  ]
}
```

### 2. 开始QQ号验证
**POST** `/api/qq/verify`

**请求参数:**
```json
{
  "qq_number": "123456789"
}
```

**响应:**
```json
{
  "success": true,
  "message": "二维码已生成，请使用QQ扫描登录验证QQ号真实性",
  "qq_number": "123456789",
  "qr_url": "/qq_login_qr.png",
  "code": 200
}
```

### 3. 查询验证状态并获取令牌
**POST** `/api/qq/status`

**请求参数:**
```json
{
  "qq_number": "123456789"
}
```

**响应 - 验证成功（返回令牌）:**
```json
{
  "success": true,
  "status": "verified",
  "message": "QQ号验证成功",
  "qq_number": "123456789",
  "verification_token": "a1b2c3d4e5f6...64位哈希令牌",
  "verified_time": 1703123456.789,
  "token_expires_minutes": 30,
  "code": 200
}
```

**响应 - 验证进行中:**
```json
{
  "success": true,
  "status": "pending",
  "message": "等待扫描二维码",
  "code": 200
}
```

**响应 - 验证失败:**
```json
{
  "success": false,
  "status": "failed",
  "message": "QQ登录失败或二维码已失效",
  "code": 401
}
```

### 4. 使用令牌绑定卡密
**POST** `/api/qq/bind`

**请求参数:**
```json
{
  "verification_token": "a1b2c3d4e5f6...64位哈希令牌",
  "card_key": "snNWnbJ3HY5NU87v"
}
```

**响应:**
```json
{
  "success": true,
  "message": "卡密绑定created成功",
  "qq_number": "123456789",
  "card_key": "snNWnbJ3HY5NU87v",
  "bind_time": "2023-12-21T10:30:45.123456",
  "code": 200
}
```

### 5. 查询QQ绑定状态
**POST** `/api/qq/query`

**请求参数:**
```json
{
  "qq_number": "123456789"
}
```

**响应 - 已绑定:**
```json
{
  "success": true,
  "bound": true,
  "data": {
    "qq_number": "123456789",
    "card_key": "snNWnbJ3HY5NU87v",
    "verification_value": 1,
    "bind_time": "2023-12-21 10:30:45",
    "last_update": "2023-12-21 10:30:45"
  },
  "code": 200
}
```

**响应 - 未绑定:**
```json
{
  "success": true,
  "bound": false,
  "message": "该QQ号未绑定卡密",
  "code": 200
}
```

### 6. 获取二维码图片
**GET** `/qq_login_qr.png`

返回PNG格式的二维码图片

## 完整使用流程

### 第一步：开始QQ号验证
```bash
curl -X POST http://localhost:5000/api/qq/verify \
  -H "Content-Type: application/json" \
  -d '{"qq_number": "123456789"}'
```

### 第二步：扫描二维码
访问 `http://localhost:5000/qq_login_qr.png` 用手机QQ扫描登录

### 第三步：轮询获取验证令牌
```bash
curl -X POST http://localhost:5000/api/qq/status \
  -H "Content-Type: application/json" \
  -d '{"qq_number": "123456789"}'
```

### 第四步：使用令牌绑定卡密
```bash
curl -X POST http://localhost:5000/api/qq/bind \
  -H "Content-Type: application/json" \
  -d '{
    "verification_token": "a1b2c3d4e5f6...",
    "card_key": "snNWnbJ3HY5NU87v"
  }'
```

## 安全机制

### 验证令牌特性
- **唯一性**：每个令牌都是唯一的SHA256哈希值
- **时效性**：30分钟有效期，过期自动失效
- **一次性**：使用后立即标记为已用，无法重复使用
- **绑定性**：令牌与特定QQ号绑定，无法跨账号使用

### 令牌生成算法
```
token_data = f"{qq_number}-{timestamp}-{random_hex}"
token = sha256(token_data).hexdigest()
```

### 防护措施
1. **QQ号格式验证**：必须为5位以上纯数字
2. **登录真实性验证**：必须能够成功登录QQ
3. **会话管理**：防止重复验证会话
4. **令牌过期清理**：自动清理过期令牌

## 错误码说明

- **200**: 成功
- **400**: 请求参数错误
- **401**: 验证失败
- **403**: 令牌无效/已使用/已过期
- **404**: 资源不存在
- **409**: 冲突（已有进行中的验证）
- **500**: 服务器内部错误

## 数据库结构

### qq_bindings 表
| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| qq_number | TEXT | QQ号（唯一） |
| card_key | TEXT | 卡密 |
| verification_value | INTEGER | 验证值（默认1） |
| bind_time | TIMESTAMP | 绑定时间 |
| last_update | TIMESTAMP | 最后更新时间 |

### verification_tokens 表
| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| qq_number | TEXT | QQ号 |
| token | TEXT | 验证令牌（唯一） |
| expires_at | TIMESTAMP | 过期时间 |
| used | INTEGER | 是否已使用（0/1） |
| created_at | TIMESTAMP | 创建时间 |

## Python 示例代码

```python
import requests
import time

class QQCardBindAPI:
    def __init__(self, base_url="http://localhost:5000"):
        self.base_url = base_url
    
    def start_verification(self, qq_number):
        """开始QQ验证"""
        response = requests.post(f"{self.base_url}/api/qq/verify", 
                               json={"qq_number": qq_number})
        return response.json()
    
    def get_verification_token(self, qq_number, max_wait=300):
        """轮询获取验证令牌"""
        start_time = time.time()
        
        while time.time() - start_time < max_wait:
            response = requests.post(f"{self.base_url}/api/qq/status",
                                   json={"qq_number": qq_number})
            result = response.json()
            
            if result.get("status") == "verified":
                return result.get("verification_token")
            elif result.get("status") == "failed":
                raise Exception(f"验证失败: {result.get('message')}")
            
            time.sleep(2)
        
        raise Exception("获取验证令牌超时")
    
    def bind_card(self, verification_token, card_key):
        """绑定卡密"""
        response = requests.post(f"{self.base_url}/api/qq/bind",
                               json={
                                   "verification_token": verification_token,
                                   "card_key": card_key
                               })
        return response.json()
    
    def query_binding(self, qq_number):
        """查询绑定状态"""
        response = requests.post(f"{self.base_url}/api/qq/query",
                               json={"qq_number": qq_number})
        return response.json()

# 使用示例
api = QQCardBindAPI()

# 1. 开始验证
qq_number = "123456789"
result = api.start_verification(qq_number)
print(f"验证开始: {result}")

# 2. 提示用户扫码
print("请扫描二维码: http://localhost:5000/qq_login_qr.png")

# 3. 获取验证令牌
try:
    token = api.get_verification_token(qq_number)
    print(f"获得验证令牌: {token}")
    
    # 4. 绑定卡密
    bind_result = api.bind_card(token, "snNWnbJ3HY5NU87v")
    print(f"绑定结果: {bind_result}")
    
except Exception as e:
    print(f"操作失败: {e}")
```

## 注意事项

1. **令牌安全**：验证令牌请妥善保管，勿泄露给他人
2. **时效限制**：令牌有30分钟有效期，请及时使用
3. **一次使用**：每个令牌只能使用一次，绑定后立即失效
4. **QQ格式**：仅支持5位以上的纯数字QQ号
5. **并发限制**：每个QQ号同时只能有一个验证会话
6. **数据持久**：绑定数据保存在SQLite数据库中