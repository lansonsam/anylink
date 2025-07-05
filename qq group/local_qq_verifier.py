#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
本地QQ验证程序
验证QQ号真实性后调用远程API绑定卡密
"""

import requests
import time
import hashlib
import secrets
from qq_group_api import QQGroupAPI
from datetime import datetime

class LocalQQVerifier:
    """本地QQ验证器"""
    
    def __init__(self, remote_api_url: str = "http://your-server.com:5000"):
        self.remote_api_url = remote_api_url
        self.api = None
        
    def generate_verification_proof(self, qq_number: str) -> str:
        """生成验证凭证"""
        timestamp = str(int(time.time()))
        random_str = secrets.token_hex(16)
        proof_data = f"{qq_number}-{timestamp}-{random_str}-LOCAL_VERIFIED"
        proof = hashlib.sha256(proof_data.encode()).hexdigest()
        return proof
    
    def verify_qq_locally(self, qq_number: str) -> tuple:
        """本地验证QQ号真实性"""
        print(f"🚀 开始本地QQ验证: {qq_number}")
        
        # 验证QQ号格式
        if not qq_number.isdigit() or len(qq_number) < 5:
            return False, "QQ号格式无效"
        
        try:
            # 初始化QQ API
            self.api = QQGroupAPI()
            print("📱 正在生成二维码...")
            
            # 生成二维码
            qrsig = self.api.initiate_qr_and_get_qrsig()
            if not qrsig:
                return False, "二维码生成失败"
            
            print("✅ 二维码已生成")
            print("📱 请使用手机QQ扫描 qq_login_qr.png 文件")
            print("⏳ 等待QQ登录验证...")
            
            # 等待扫码登录
            login_successful = self.api.poll_scan_status_and_login(qrsig)
            
            if not login_successful:
                return False, "QQ登录失败或二维码过期"
            
            print("✅ QQ登录成功，验证通过！")
            return True, "验证成功"
            
        except Exception as e:
            return False, f"验证过程出错: {str(e)}"
    
    def bind_card_to_remote(self, qq_number: str, card_key: str, verification_proof: str) -> dict:
        """调用远程API绑定卡密"""
        print(f"🌐 正在连接远程API绑定卡密...")
        
        try:
            # 发送绑定请求到远程服务器
            response = requests.post(
                f"{self.remote_api_url}/api/bind_with_proof",
                json={
                    "qq_number": qq_number,
                    "card_key": card_key,
                    "verification_proof": verification_proof,
                    "verified_time": datetime.now().isoformat(),
                    "verifier_type": "local_qq_verifier"
                },
                timeout=30
            )
            
            return response.json()
            
        except requests.exceptions.ConnectionError:
            return {"error": "无法连接到远程服务器"}
        except requests.exceptions.Timeout:
            return {"error": "请求超时"}
        except Exception as e:
            return {"error": f"请求失败: {str(e)}"}
    
    def complete_verification_and_binding(self, qq_number: str, card_key: str) -> bool:
        """完整的验证和绑定流程"""
        print("=" * 60)
        print("🎯 本地QQ验证 + 远程卡密绑定")
        print("=" * 60)
        
        # 1. 本地验证QQ
        success, message = self.verify_qq_locally(qq_number)
        if not success:
            print(f"❌ 本地验证失败: {message}")
            return False
        
        # 2. 生成验证凭证
        verification_proof = self.generate_verification_proof(qq_number)
        print(f"🔑 生成验证凭证: {verification_proof[:20]}...")
        
        # 3. 调用远程API绑定
        result = self.bind_card_to_remote(qq_number, card_key, verification_proof)
        
        if result.get("success"):
            print("✅ 远程绑定成功！")
            print(f"👤 QQ号: {qq_number}")
            print(f"💳 卡密: {card_key}")
            print(f"🕒 绑定时间: {result.get('bind_time')}")
            return True
        else:
            print(f"❌ 远程绑定失败: {result.get('error', '未知错误')}")
            return False

def main():
    """主程序"""
    print("🌟 本地QQ验证器 + 远程卡密绑定")
    print("=" * 40)
    
    # 配置
    REMOTE_API_URL = "http://your-server.com:5000"  # 修改为您的远程服务器地址
    
    try:
        # 用户输入
        qq_number = input("🔢 请输入QQ号: ").strip()
        card_key = input("💳 请输入卡密: ").strip()
        
        if not qq_number or not card_key:
            print("❌ QQ号和卡密不能为空")
            return
        
        # 询问远程服务器地址
        custom_url = input(f"🌐 远程API地址 (默认: {REMOTE_API_URL}): ").strip()
        if custom_url:
            REMOTE_API_URL = custom_url
        
        # 创建验证器
        verifier = LocalQQVerifier(REMOTE_API_URL)
        
        # 执行验证和绑定
        success = verifier.complete_verification_and_binding(qq_number, card_key)
        
        if success:
            print("\n🎉 所有操作完成！")
        else:
            print("\n❌ 操作失败")
            
    except KeyboardInterrupt:
        print("\n👋 用户取消操作")
    except Exception as e:
        print(f"\n❌ 程序异常: {e}")

if __name__ == "__main__":
    main() 