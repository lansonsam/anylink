#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
牛子公益API - 简单客户端
快速测试QQ验证和卡密绑定
"""

import requests
import time

# 配置
API_URL = "http://localhost:5000"
QQ_NUMBER = "123456789"  # 修改为您的QQ号
CARD_KEY = "snNWnbJ3HY5NU87v"  # 修改为您的卡密

def test_api():
    """测试API流程"""
    print("=" * 50)
    print("🌟 牛子公益API 测试")
    print("=" * 50)
    
    # 1. 检查API状态
    print("\n1️⃣ 检查API状态...")
    try:
        response = requests.get(f"{API_URL}/")
        if response.status_code == 200:
            data = response.json()
            print(f"✅ API服务正常: {data.get('api_name')} v{data.get('version')}")
        else:
            print(f"❌ API错误: {response.status_code}")
            return
    except Exception as e:
        print(f"❌ 连接失败: {e}")
        print("💡 请确保API服务已启动 (python app.py)")
        return
    
    # 2. 开始QQ验证
    print(f"\n2️⃣ 开始QQ验证: {QQ_NUMBER}")
    try:
        response = requests.post(f"{API_URL}/api/qq/verify", 
                               json={"qq_number": QQ_NUMBER})
        data = response.json()
        
        if data.get("success"):
            print(f"✅ {data.get('message')}")
            print(f"📱 二维码: {API_URL}/qq_login_qr.png")
            print("💡 请用手机QQ扫描上述链接的二维码登录")
        else:
            print(f"❌ 验证启动失败: {data.get('error')}")
            return
    except Exception as e:
        print(f"❌ 请求失败: {e}")
        return
    
    # 3. 等待验证完成
    print(f"\n3️⃣ 等待QQ登录验证...")
    verification_token = None
    max_wait = 180  # 3分钟
    start_time = time.time()
    
    while time.time() - start_time < max_wait:
        try:
            response = requests.post(f"{API_URL}/api/qq/status",
                                   json={"qq_number": QQ_NUMBER})
            data = response.json()
            
            status = data.get("status")
            message = data.get("message")
            
            elapsed = int(time.time() - start_time)
            
            if status == "verified":
                verification_token = data.get("verification_token")
                print(f"✅ 验证成功！获得令牌: {verification_token[:20]}...")
                break
            elif status == "failed":
                print(f"❌ 验证失败: {message}")
                return
            else:
                print(f"⏳ {message} (已等待{elapsed}秒)")
            
            time.sleep(3)
            
        except Exception as e:
            print(f"❌ 状态查询失败: {e}")
            return
    
    if not verification_token:
        print("⏰ 等待超时，请重试")
        return
    
    # 4. 绑定卡密
    print(f"\n4️⃣ 绑定卡密: {CARD_KEY}")
    try:
        response = requests.post(f"{API_URL}/api/qq/bind",
                               json={
                                   "verification_token": verification_token,
                                   "card_key": CARD_KEY
                               })
        data = response.json()
        
        if data.get("success"):
            print(f"✅ {data.get('message')}")
            print(f"👤 QQ号: {data.get('qq_number')}")
            print(f"💳 卡密: {data.get('card_key')}")
            print(f"🕒 绑定时间: {data.get('bind_time')}")
        else:
            print(f"❌ 绑定失败: {data.get('error')}")
            return
    except Exception as e:
        print(f"❌ 绑定请求失败: {e}")
        return
    
    # 5. 查询绑定状态
    print(f"\n5️⃣ 验证绑定结果...")
    try:
        response = requests.post(f"{API_URL}/api/qq/query",
                               json={"qq_number": QQ_NUMBER})
        data = response.json()
        
        if data.get("bound"):
            bind_data = data.get("data", {})
            print("✅ 绑定验证成功！")
            print(f"👤 QQ号: {bind_data.get('qq_number')}")
            print(f"💳 卡密: {bind_data.get('card_key')}")
            print(f"🔢 验证值: {bind_data.get('verification_value')}")
            print(f"🕒 绑定时间: {bind_data.get('bind_time')}")
        else:
            print("❌ 未找到绑定记录")
    except Exception as e:
        print(f"❌ 查询失败: {e}")
    
    print("\n" + "=" * 50)
    print("🎉 测试完成！")
    print("=" * 50)

def quick_query(qq_number=None):
    """快速查询QQ绑定状态"""
    qq = qq_number or QQ_NUMBER
    print(f"🔍 查询QQ绑定状态: {qq}")
    
    try:
        response = requests.post(f"{API_URL}/api/qq/query",
                               json={"qq_number": qq})
        data = response.json()
        
        if data.get("bound"):
            bind_data = data.get("data", {})
            print("✅ 已绑定")
            print(f"💳 卡密: {bind_data.get('card_key')}")
            print(f"🕒 绑定时间: {bind_data.get('bind_time')}")
        else:
            print("❌ 未绑定")
    except Exception as e:
        print(f"❌ 查询失败: {e}")

if __name__ == "__main__":
    import sys
    
    if len(sys.argv) > 1:
        if sys.argv[1] == "query":
            qq = sys.argv[2] if len(sys.argv) > 2 else None
            quick_query(qq)
        else:
            print("用法: python simple_client.py [query] [qq_number]")
    else:
        print("🚀 开始完整测试流程...")
        print(f"📝 QQ号: {QQ_NUMBER}")
        print(f"💳 卡密: {CARD_KEY}")
        print(f"🌐 API: {API_URL}")
        print("\n💡 如需修改参数，请编辑文件顶部的配置")
        
        input("\n按回车键开始测试...")
        test_api() 