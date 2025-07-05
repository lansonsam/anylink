# 牛子公益API - 本地客户端使用说明

## 概述
这是牛子公益API的本地Python客户端，用于演示QQ验证令牌绑定卡密的完整流程。

## 文件说明

### 1. `client_example.py` - 完整功能客户端
- 交互式界面
- 完整错误处理
- 自动打开二维码页面
- 详细状态显示

### 2. `simple_client.py` - 简化测试客户端
- 快速测试流程
- 预设参数配置
- 命令行支持

## 使用方法

### 准备工作

1. **启动API服务**
```bash
python app.py
```

2. **安装依赖**
```bash
pip install requests
```

### 方法一：使用完整客户端

运行交互式客户端：
```bash
python client_example.py
```

按提示输入QQ号和卡密，然后扫描二维码完成验证。

### 方法二：使用简化客户端

1. **修改配置参数**
编辑 `simple_client.py` 文件顶部：
```python
QQ_NUMBER = "您的QQ号"     # 修改为实际QQ号
CARD_KEY = "您的卡密"      # 修改为实际卡密
```

2. **运行完整测试**
```bash
python simple_client.py
```

3. **快速查询绑定状态**
```bash
# 查询默认QQ号
python simple_client.py query

# 查询指定QQ号
python simple_client.py query 123456789
```

## 流程说明

### 完整验证绑定流程

1. **开始验证** → 调用 `/api/qq/verify`
   - 输入QQ号
   - 系统生成二维码

2. **扫描登录** → 手机QQ扫码
   - 访问 `http://localhost:5000/qq_login_qr.png`
   - 使用手机QQ扫描登录

3. **获取令牌** → 轮询 `/api/qq/status`
   - 客户端自动等待验证完成
   - 获得30分钟有效期的验证令牌

4. **绑定卡密** → 调用 `/api/qq/bind`
   - 使用令牌绑定卡密
   - 令牌一次性使用后失效

5. **验证结果** → 调用 `/api/qq/query`
   - 确认绑定成功
   - 查看绑定信息

## 示例输出

### 成功流程输出
```
==================================================
🌟 牛子公益API 测试
==================================================

1️⃣ 检查API状态...
✅ API服务正常: 牛子公益API v2.0

2️⃣ 开始QQ验证: 123456789
✅ 二维码已生成，请使用QQ扫描登录验证QQ号真实性
📱 二维码: http://localhost:5000/qq_login_qr.png
💡 请用手机QQ扫描上述链接的二维码登录

3️⃣ 等待QQ登录验证...
⏳ 等待扫描二维码 (已等待3秒)
⏳ 扫码成功，正在验证QQ号 (已等待6秒)
✅ 验证成功！获得令牌: a1b2c3d4e5f6789012...

4️⃣ 绑定卡密: snNWnbJ3HY5NU87v
✅ 卡密绑定created成功
👤 QQ号: 123456789
💳 卡密: snNWnbJ3HY5NU87v
🕒 绑定时间: 2023-12-21T10:30:45.123456

5️⃣ 验证绑定结果...
✅ 绑定验证成功！
👤 QQ号: 123456789
💳 卡密: snNWnbJ3HY5NU87v
🔢 验证值: 1
🕒 绑定时间: 2023-12-21 10:30:45

==================================================
🎉 测试完成！
==================================================
```

## 错误处理

### 常见错误及解决方案

1. **连接失败**
```
❌ 连接失败: [Errno 111] Connection refused
💡 请确保API服务已启动 (python app.py)
```
**解决**：启动API服务

2. **QQ号格式错误**
```
❌ QQ号格式无效（必须为5位以上数字）
```
**解决**：使用正确的QQ号格式

3. **验证超时**
```
⏰ 等待超时，请重试
```
**解决**：重新运行，确保及时扫描二维码

4. **令牌已使用**
```
❌ 绑定失败: Token已被使用
```
**解决**：重新进行QQ验证获取新令牌

## 自定义配置

### 修改API地址
```python
API_URL = "http://your-server:5000"  # 修改为实际服务地址
```

### 调整等待时间
```python
max_wait = 300  # 修改最大等待时间（秒）
check_interval = 3  # 修改状态检查间隔（秒）
```

## API调用示例

### 基础调用代码
```python
import requests

# 1. 开始验证
response = requests.post("http://localhost:5000/api/qq/verify", 
                        json={"qq_number": "123456789"})
print(response.json())

# 2. 查询状态
response = requests.post("http://localhost:5000/api/qq/status",
                        json={"qq_number": "123456789"})
result = response.json()

if result.get("status") == "verified":
    token = result.get("verification_token")
    
    # 3. 绑定卡密
    response = requests.post("http://localhost:5000/api/qq/bind",
                            json={
                                "verification_token": token,
                                "card_key": "snNWnbJ3HY5NU87v"
                            })
    print(response.json())
```

## 注意事项

1. **API服务必须先启动**：确保 `python app.py` 已运行
2. **令牌时效性**：验证令牌有30分钟有效期，请及时使用
3. **一次性使用**：每个令牌只能使用一次
4. **网络连接**：确保客户端能访问API服务地址
5. **QQ登录**：必须能够正常登录QQ才能完成验证

## 技术支持

如遇问题，请检查：
- API服务是否正常运行
- 网络连接是否正常
- QQ号格式是否正确
- 是否及时扫描二维码

---

**牛子公益API v2.0**  
QQ验证令牌绑定卡密系统 