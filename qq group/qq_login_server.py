#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
QQ登录服务器集成脚本
用于OIDC提供者的QQ登录功能
"""

import sys
import json
import requests
import time
from qq_group_api import QQGroupAPI

def update_session_status(session_id, status, data=None):
    """更新会话状态到Go服务器"""
    try:
        payload = {
            "session_id": session_id,
            "status": status
        }
        
        if data:
            payload.update(data)
        
        response = requests.post(
            "http://localhost:8080/qq/callback",
            json=payload,
            timeout=5
        )
        
        return response.status_code == 200
    except Exception as e:
        print(f"更新状态失败: {e}")
        return False

def handle_qq_login(session_id):
    """处理QQ登录流程"""
    print(f"开始QQ登录流程，会话ID: {session_id}")
    
    try:
        # 初始化QQ API
        api = QQGroupAPI()
        
        # 生成二维码
        print("正在生成二维码...")
        qrsig = api.initiate_qr_and_get_qrsig()
        
        if not qrsig:
            update_session_status(session_id, "failed", {"error": "二维码生成失败"})
            return False
        
        print("二维码已生成: qq_login_qr.png")
        
        # 创建一个修改版的轮询函数，可以更新状态
        original_poll = api.poll_scan_status_and_login
        
        def custom_poll(qrsig):
            """自定义轮询函数，增加状态更新"""
            qr_file = "qq_login_qr.png"
            try:
                print("等待用户扫描二维码...")
                qr_state_url = "https://ssl.ptlogin2.qq.com/ptqrlogin"
                ptqrtoken = api._get_ptqrtoken(qrsig)
                
                while True:
                    current_timestamp_ms = str(int(time.time() * 1000))
                    state_params = {
                        "u1": "https://qun.qq.com/member.html",
                        "ptqrtoken": str(ptqrtoken),
                        "ptredirect": "0",
                        "h": "1",
                        "t": "1",
                        "g": "1",
                        "from_ui": "1",
                        "ptlang": "2052",
                        "action": f"0-0-{current_timestamp_ms}",
                        "js_ver": "24051615",
                        "js_type": "1",
                        "pt_uistyle": "40",
                        "aid": api.login_params["appid"],
                        "daid": api.login_params["daid"],
                        "pt_3rd_aid": api.login_params["pt_3rd_aid"],
                    }
                    
                    login_sig_from_cookie = api.session.cookies.get("pt_login_sig")
                    if login_sig_from_cookie:
                        state_params["login_sig"] = login_sig_from_cookie
                    
                    state_response = api.session.get(qr_state_url, params=state_params)
                    response_text = state_response.text
                    
                    if "ptuiCB('0','0'" in response_text or "登录成功" in response_text:
                        print("登录成功！")
                        # 尝试从响应中提取昵称
                        import re
                        nickname_match = re.search(r"'([^']+)'\)$", response_text)
                        if nickname_match:
                            api.nickname = nickname_match.group(1)
                        # 清理二维码文件
                        if os.path.exists(qr_file):
                            try:
                                os.remove(qr_file)
                            except:
                                pass
                        return True
                    elif "ptuiCB('65'" in response_text:
                        print("二维码已扫描，等待确认...")
                        update_session_status(session_id, "scanning")
                    elif "ptuiCB('67'" in response_text or "二维码未失效" in response_text:
                        print("等待扫码...")
                    elif "ptuiCB('66'" in response_text:
                        print("二维码未失效，请扫描...")
                    elif "ptuiCB('10009'" in response_text or "ptuiCB('10006'" in response_text or "二维码已失效" in response_text:
                        print("二维码已失效")
                        update_session_status(session_id, "failed", {"error": "二维码已失效"})
                        return False
                    
                    time.sleep(2)
                    
            except Exception as e:
                print(f"轮询失败: {e}")
                update_session_status(session_id, "failed", {"error": str(e)})
                return False
        
        # 使用自定义轮询函数
        login_success = custom_poll(qrsig)
        
        if not login_success:
            return False
        
        # 更新状态为已确认
        update_session_status(session_id, "confirmed")
        
        # 获取QQ号和昵称
        # 尝试从cookies中获取QQ号
        qq_number = "unknown"
        nickname = getattr(api, 'nickname', 'QQ用户')
        
        # 尝试从cookie中获取uin（QQ号）
        for cookie in api.session.cookies:
            if cookie.name == 'uin' or cookie.name == 'p_uin':
                # QQ号通常以o开头，需要去掉
                uin_value = cookie.value
                if uin_value.startswith('o'):
                    qq_number = uin_value[1:].lstrip('0')  # 去掉o和前导0
                else:
                    qq_number = uin_value
                break
        
        print(f"QQ验证成功: {qq_number}")
        
        # 更新为完成状态
        update_session_status(session_id, "completed", {
            "qq_number": qq_number,
            "nickname": nickname
        })
        
        print("QQ登录流程完成！")
        return True
        
    except Exception as e:
        print(f"QQ登录失败: {e}")
        update_session_status(session_id, "failed", {"error": str(e)})
        return False

def main():
    """主函数"""
    if len(sys.argv) < 2:
        print("用法: python qq_login_server.py <session_id>")
        sys.exit(1)
    
    session_id = sys.argv[1]
    
    # 处理QQ登录
    success = handle_qq_login(session_id)
    
    if success:
        print("登录成功")
        sys.exit(0)
    else:
        print("登录失败")
        sys.exit(1)

if __name__ == "__main__":
    import os
    main()