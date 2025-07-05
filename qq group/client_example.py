#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
牛子公益API - 本地客户端示例
QQ验证令牌绑定卡密系统
"""

import requests
import time
import json
import webbrowser
from typing import Optional, Dict, Any

class QQCardBindClient:
    """QQ绑定卡密客户端"""
    
    def __init__(self, api_base_url: str = "http://localhost:5000"):
        self.api_url = api_base_url
        self.session = requests.Session()
        self.session.headers.update({
            'Content-Type': 'application/json',
            'User-Agent': 'QQCardBindClient/1.0'
        })
    
    def _make_request(self, method: str, endpoint: str, data: Optional[Dict] = None) -> Dict[Any, Any]:
        """发送HTTP请求"""
        url = f"{self.api_url}{endpoint}"
        
        try:
            if method.upper() == 'GET':
                response = self.session.get(url)
            elif method.upper() == 'POST':
                response = self.session.post(url, json=data)
            else:
                raise ValueError(f"不支持的HTTP方法: {method}")
            
            # 检查HTTP状态
            if response.status_code >= 400:
                print(f"❌ HTTP错误 {response.status_code}: {response.text}")
                return {"error": f"HTTP {response.status_code}"}
            
            return response.json()
            
        except requests.exceptions.ConnectionError:
            return {"error": "无法连接到API服务器，请确保服务已启动"}
        except requests.exceptions.Timeout:
            return {"error": "请求超时"}
        except Exception as e:
            return {"error": f"请求失败: {str(e)}"}
    
    def check_api_status(self) -> bool:
        """检查API服务状态"""
        print("🔍 检查API服务状态...")
        result = self._make_request('GET', '/')
        
        if 'error' in result:
            print(f"❌ API服务不可用: {result['error']}")
            return False
        
        print(f"✅ API服务正常: {result.get('api_name', 'Unknown')} v{result.get('version', '?')}")
        return True
    
    def start_qq_verification(self, qq_number: str) -> bool:
        """开始QQ验证"""
        print(f"\n🚀 开始QQ号验证: {qq_number}")
        
        # 验证QQ号格式
        if not qq_number.isdigit() or len(qq_number) < 5:
            print("❌ QQ号格式无效（必须为5位以上数字）")
            return False
        
        # 发送验证请求
        result = self._make_request('POST', '/api/qq/verify', {
            'qq_number': qq_number
        })
        
        if 'error' in result:
            print(f"❌ 验证启动失败: {result['error']}")
            return False
        
        if result.get('success'):
            print(f"✅ {result.get('message', '验证已启动')}")
            print(f"📱 二维码地址: {self.api_url}{result.get('qr_url', '/qq_login_qr.png')}")
            
            # 尝试自动打开二维码
            try:
                qr_url = f"{self.api_url}/qq_login_qr.png"
                print(f"🌐 正在尝试打开二维码页面...")
                webbrowser.open(qr_url)
            except:
                print("💡 请手动访问二维码链接扫描登录")
            
            return True
        else:
            print(f"❌ 验证启动失败: {result}")
            return False
    
    def wait_for_verification_token(self, qq_number: str, max_wait_seconds: int = 300) -> Optional[str]:
        """等待并获取验证令牌"""
        print(f"\n⏳ 等待QQ验证完成（最长等待{max_wait_seconds}秒）...")
        print("📱 请使用手机QQ扫描二维码并登录")
        
        start_time = time.time()
        check_interval = 3  # 每3秒检查一次
        
        while time.time() - start_time < max_wait_seconds:
            # 查询验证状态
            result = self._make_request('POST', '/api/qq/status', {
                'qq_number': qq_number
            })
            
            if 'error' in result:
                print(f"❌ 状态查询失败: {result['error']}")
                return None
            
            status = result.get('status')
            message = result.get('message', '')
            
            if status == 'verified':
                token = result.get('verification_token')
                expires_minutes = result.get('token_expires_minutes', 30)
                print(f"✅ QQ验证成功！")
                print(f"🔑 验证令牌: {token[:20]}...（已获取）")
                print(f"⏰ 令牌有效期: {expires_minutes}分钟")
                return token
                
            elif status == 'failed':
                print(f"❌ QQ验证失败: {message}")
                return None
                
            elif status == 'pending':
                # 显示进度
                elapsed = int(time.time() - start_time)
                remaining = max_wait_seconds - elapsed
                print(f"⏳ {message} (已等待{elapsed}秒，剩余{remaining}秒)")
            
            time.sleep(check_interval)
        
        print("⏰ 等待超时，验证失败")
        return None
    
    def bind_card_key(self, verification_token: str, card_key: str) -> bool:
        """使用验证令牌绑定卡密"""
        print(f"\n💳 开始绑定卡密: {card_key}")
        
        result = self._make_request('POST', '/api/qq/bind', {
            'verification_token': verification_token,
            'card_key': card_key
        })
        
        if 'error' in result:
            print(f"❌ 绑定失败: {result['error']}")
            return False
        
        if result.get('success'):
            qq_number = result.get('qq_number')
            bind_time = result.get('bind_time')
            print(f"✅ {result.get('message', '绑定成功')}")
            print(f"👤 QQ号: {qq_number}")
            print(f"💳 卡密: {card_key}")
            print(f"🕒 绑定时间: {bind_time}")
            return True
        else:
            print(f"❌ 绑定失败: {result}")
            return False
    
    def query_binding_status(self, qq_number: str) -> Optional[Dict]:
        """查询QQ绑定状态"""
        print(f"\n🔍 查询QQ绑定状态: {qq_number}")
        
        result = self._make_request('POST', '/api/qq/query', {
            'qq_number': qq_number
        })
        
        if 'error' in result:
            print(f"❌ 查询失败: {result['error']}")
            return None
        
        if result.get('bound'):
            data = result.get('data', {})
            print("✅ 该QQ号已绑定卡密")
            print(f"👤 QQ号: {data.get('qq_number')}")
            print(f"💳 卡密: {data.get('card_key')}")
            print(f"🔢 验证值: {data.get('verification_value')}")
            print(f"🕒 绑定时间: {data.get('bind_time')}")
            print(f"🔄 更新时间: {data.get('last_update')}")
            return data
        else:
            print("❌ 该QQ号未绑定卡密")
            return None
    
    def complete_binding_flow(self, qq_number: str, card_key: str) -> bool:
        """完整的绑定流程"""
        print("=" * 60)
        print("🎯 开始完整的QQ号验证绑定流程")
        print("=" * 60)
        
        # 1. 检查API状态
        if not self.check_api_status():
            return False
        
        # 2. 开始QQ验证
        if not self.start_qq_verification(qq_number):
            return False
        
        # 3. 等待验证令牌
        token = self.wait_for_verification_token(qq_number)
        if not token:
            return False
        
        # 4. 绑定卡密
        if not self.bind_card_key(token, card_key):
            return False
        
        # 5. 验证绑定结果
        self.query_binding_status(qq_number)
        
        print("\n" + "=" * 60)
        print("🎉 绑定流程完成！")
        print("=" * 60)
        return True


def main():
    """主程序"""
    print("🌟 牛子公益API - 本地客户端示例")
    print("QQ验证令牌绑定卡密系统\n")
    
    # 创建客户端实例
    client = QQCardBindClient()
    
    # 交互式输入
    try:
        print("请输入以下信息：")
        qq_number = input("🔢 QQ号: ").strip()
        card_key = input("💳 卡密: ").strip()
        
        if not qq_number or not card_key:
            print("❌ QQ号和卡密不能为空")
            return
        
        # 询问是否查询现有绑定
        check_existing = input("\n🤔 是否先查询该QQ号的现有绑定状态？(y/n): ").strip().lower()
        if check_existing == 'y':
            client.query_binding_status(qq_number)
            
            continue_bind = input("\n🤔 是否继续进行绑定？(y/n): ").strip().lower()
            if continue_bind != 'y':
                print("👋 已取消操作")
                return
        
        # 执行完整绑定流程
        success = client.complete_binding_flow(qq_number, card_key)
        
        if success:
            print("✅ 所有操作完成成功！")
        else:
            print("❌ 操作失败，请检查错误信息")
            
    except KeyboardInterrupt:
        print("\n👋 用户取消操作")
    except Exception as e:
        print(f"\n❌ 程序异常: {e}")


if __name__ == "__main__":
    main() 