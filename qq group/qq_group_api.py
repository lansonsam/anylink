import requests
from typing import List, Dict, Optional, Tuple
from dataclasses import dataclass
import json
import time
import qrcode
import os
from PIL import Image
import re

@dataclass
class QQGroup:
    group_name: str
    group_id: str
    member_count: Optional[int] = None
    
class QQGroupAPI:
    def __init__(self):
        self.session = requests.Session()
        self.base_url = "https://qun.qq.com"
        self.headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
            "Accept": "application/json, text/plain, */*",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
            "Referer": "https://qun.qq.com/member.html",
            "Origin": "https://qun.qq.com"
        }
        self.session.headers.update(self.headers)
        self.login_params = {
            "appid": "715030901",
            "daid": "73",
            "pt_3rd_aid": "0",
        }

    def _get_ptqrtoken(self, qrsig: str) -> int:
        """
        计算ptqrtoken
        :param qrsig: qrsig cookie值
        :return: ptqrtoken值
        """
        hash_val = 0
        for char in qrsig:
            hash_val += (hash_val << 5) + ord(char)
        return hash_val & 2147483647

    def initiate_qr_and_get_qrsig(self) -> Optional[str]:
        """
        初始化QQ扫码登录，获取并保存二维码图片，返回qrsig。
        :return: qrsig字符串如果成功，否则返回None
        """
        try:
            print("正在初始化登录...")
            # 1. 访问群管理页面获取xlogin URL (此步骤可能不是严格必须，但保留原逻辑)
            # self.session.get("https://qun.qq.com/member.html") # May set some initial cookies
            
            xlogin_url = f"https://xui.ptlogin2.qq.com/cgi-bin/xlogin?appid={self.login_params['appid']}&daid={self.login_params['daid']}&s=8&pt_3rd_aid=0"
            
            print("正在访问xlogin页面获取登录参数...")
            # 2. 访问xlogin页面获取必要参数 (pt_login_sig等，可能通过cookies设置)
            self.session.get(xlogin_url) 
            
            # 3. 获取二维码
            print("正在获取二维码...")
            qr_code_url = "https://ssl.ptlogin2.qq.com/ptqrshow"
            qr_params = {
                "appid": self.login_params["appid"],
                "e": "2",
                "l": "M",
                "s": "3",
                "d": "72",
                "v": "4",
                "t": str(time.time()),
                "daid": self.login_params["daid"],
                "pt_3rd_aid": self.login_params["pt_3rd_aid"],
            }
            qr_response = self.session.get(qr_code_url, params=qr_params)

            if qr_response.status_code != 200:
                print("获取二维码失败")
                return None

            # 保存二维码图片
            qr_file = "qq_login_qr.png"
            with open(qr_file, "wb") as f:
                f.write(qr_response.content)
            print(f"二维码已保存至 {qr_file}")

            qrsig = qr_response.cookies.get("qrsig")
            if not qrsig:
                print("获取qrsig失败")
                if os.path.exists(qr_file): # 清理无效二维码
                    os.remove(qr_file)
                return None
            
            return qrsig

        except Exception as e:
            print(f"初始化二维码获取失败: {str(e)}")
            return None

    def poll_scan_status_and_login(self, qrsig: str) -> bool:
        """
        轮询二维码扫描状态并完成登录。
        :param qrsig: 从 initiate_qr_and_get_qrsig 获取的qrsig
        :return: 是否登录成功
        """
        if not qrsig:
            print("qrsig为空，无法检查扫描状态")
            return False
            
        qr_file = "qq_login_qr.png" # 用于登录成功后删除

        try:
            print("请使用手机QQ扫描二维码登录...")
            qr_state_url = "https://ssl.ptlogin2.qq.com/ptqrlogin"
            ptqrtoken = self._get_ptqrtoken(qrsig)

            while True:
                current_timestamp_ms = str(int(time.time() * 1000)) # 根据观察到的请求，action参数是精确到毫秒的时间戳
                state_params = {
                    "u1": "https://qun.qq.com/member.html",
                    "ptqrtoken": str(ptqrtoken),
                    "ptredirect": "0",
                    "h": "1",
                    "t": "1", # 和之前获取二维码的t不一样，这里通常是1
                    "g": "1",
                    "from_ui": "1",
                    "ptlang": "2052", # 简体中文
                    "action": f"0-0-{current_timestamp_ms}", # 格式如 "0-0-1622334455667"
                    "js_ver": "24051615", # 这个版本号可能会变，使用一个较新的
                    "js_type": "1", # 保持为1
                    # "login_sig": "", # login_sig可能在xlogin时获取并设置在session.cookies中，这里可能不需要显式传递
                    "pt_uistyle": "40",
                    "aid": self.login_params["appid"],
                    "daid": self.login_params["daid"],
                    "pt_3rd_aid": self.login_params["pt_3rd_aid"], # 确保这个参数也被传递
                    # "mibao_css": "m_webqq", # 可选参数，视情况添加
                }
                
                # 从session.cookies中获取login_sig
                login_sig_from_cookie = self.session.cookies.get("pt_login_sig")
                if login_sig_from_cookie:
                    state_params["login_sig"] = login_sig_from_cookie

                state_response = self.session.get(qr_state_url, params=state_params)
                response_text = state_response.text
                # print(f"二维码状态响应: {response_text}") # 调试用

                if "ptuiCB('0','0'" in response_text or "登录成功" in response_text or "ptuiCB('65'" in response_text: # '65' 是已扫描状态
                    # 'ptuiCB('0','0','https://ssl.ptlogin2.qq.com/check_sig?...','0','登录成功！', '昵称')'
                    # 'ptuiCB('65','0','','0','二维码已扫描，请在手机上确认登录。', '昵称')'
                    if "登录成功" in response_text or "ptuiCB('0','0'" in response_text: # 确认是登录成功
                        print("登录成功！")
                        match = re.search(r"ptuiCB\('0','0','([^']+)','0'", response_text)
                        if match:
                            redirect_url = match.group(1)
                            print(f"正在跳转到：{redirect_url}")
                            
                            # 访问跳转URL获取必要的Cookie (如 skey, p_skey, pt4_token)
                            # 需要特别注意这里的 referer 和 origin，腾讯的检查比较严格
                            redirect_headers = self.headers.copy()
                            redirect_headers["Referer"] = "https://ssl.ptlogin2.qq.com/" # 跳转前的 Referer
                            # redirect_headers["Origin"] is not usually sent for GET navigation
                            
                            # 移除可能冲突的 Content-Type
                            if "Content-Type" in redirect_headers:
                                del redirect_headers["Content-Type"]

                            # 允许重定向，让requests库自动处理
                            response = self.session.get(redirect_url, headers=redirect_headers, allow_redirects=True)
                            
                            # 检查关键Cookie是否存在
                            required_cookies = ['skey', 'p_skey'] # pt4_token 有时不是立即获得或非必须
                            missing_cookies = [cookie for cookie in required_cookies if not self.session.cookies.get(cookie)]
                            
                            if missing_cookies:
                                print(f"缺少必要的Cookie: {', '.join(missing_cookies)}")
                                print(f"当前所有Cookies: {self.session.cookies.get_dict()}")
                                # 即使缺少部分cookie，也可能能继续操作，这里不直接返回False
                                # return False 
                            
                            # 登录成功后，可能需要再次访问群管理页面来确认会话有效性或获取额外cookies
                            # self.session.get("https://qun.qq.com/member.html") 
                            
                            if os.path.exists(qr_file):
                                try:
                                    os.remove(qr_file)
                                    print(f"已清理二维码文件: {qr_file}")
                                except OSError as e:
                                    print(f"清理二维码文件失败: {e}")
                            return True
                        else:
                            print("无法从响应中获取跳转URL")
                            print(f"完整响应: {response_text}")
                            return False
                    elif "ptuiCB('65'" in response_text: # '65' 是已扫描，等待确认
                        print("二维码已扫描，请在手机上确认登录...")
                    else: # 其他情况，比如 '67' 是继续等待 '66' 是未失效
                        print("等待扫码或确认...")

                elif "ptuiCB('67'" in response_text or "二维码未失效" in response_text: # '67' 是继续等待 '66' 是未失效
                    print("等待扫码...")
                elif "ptuiCB('66'" in response_text:
                     print("二维码未失效，请扫描...")
                elif "ptuiCB('10009'" in response_text or "ptuiCB('10006'" in response_text or "二维码已失效" in response_text:
                    print("二维码已失效，请重新生成。")
                    if os.path.exists(qr_file):
                        try:
                            os.remove(qr_file)
                        except OSError as e:
                            print(f"清理二维码文件失败: {e}")
                    return False
                else:
                    print(f"未知的二维码状态: {response_text}")
                    # 持续未知状态可能也意味着失效或需要重新开始
                    # return False # 暂时不因为未知状态而直接失败

                time.sleep(2) # 轮询间隔

        except Exception as e:
            print(f"轮询二维码状态或登录失败: {str(e)}")
            if os.path.exists(qr_file): # 发生异常时也尝试清理
                try:
                    os.remove(qr_file)
                except OSError as err:
                    print(f"异常后清理二维码文件失败: {err}")
            return False

    def get_joined_groups(self) -> List[QQGroup]:
        """
        获取已加入的QQ群列表
        :return: QQ群列表
        """
        try:
            url = f"{self.base_url}/cgi-bin/qun_mgr/get_group_list"
            bkn = self._get_g_tk() # g_tk/bkn
            
            # 更新请求头
            get_group_headers = {
                "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
                "Origin": "https://qun.qq.com",
                "Referer": "https://qun.qq.com/member.html" # 确保 Referer 正确
            }
            # self.session.headers.update(get_group_headers) # 不应该全局更新，而是针对特定请求

            data = {
                "bkn": bkn
            }
            
            print(f"获取群列表请求URL: {url}")
            print(f"bkn: {bkn}")
            # print(f"请求头: {self.session.headers}") # 打印的是 session 的全局头
            # print(f"Cookie: {self.session.cookies.get_dict()}")
            
            response = self.session.post(url, data=data, headers=get_group_headers) # 为本次请求传递特定头
            print(f"获取群列表响应状态码: {response.status_code}")
            # print(f"获取群列表响应内容: {response.text}") # 避免打印过多信息
            
            if response.status_code == 200:
                try:
                    response_data = response.json()
                    # 有时返回的是字符串包裹的JSON
                    if isinstance(response_data, str):
                        try:
                            response_data = json.loads(response_data)
                        except json.JSONDecodeError:
                            print(f"尝试解析字符串JSON失败: {response_data[:200]}...") # 只打印一部分
                            return []
                        
                    # print(f"解析后的JSON数据: {json.dumps(response_data, ensure_ascii=False, indent=2)}")
                    
                    if response_data.get("ec") == 0: # ec=0 表示成功
                        groups = []
                        for group_type in ['create', 'manage', 'join']: # 确保覆盖所有可能的群组类型
                            group_list = response_data.get(group_type, [])
                            if group_list: # 确保 group_list 不是 None
                                for group_info in group_list:
                                    if isinstance(group_info, dict): # 确保 group_info 是字典
                                        groups.append(QQGroup(
                                            group_name=group_info.get("gn", "未知群名"),
                                            group_id=str(group_info.get("gc", "")),
                                            member_count=group_info.get("goc", 0) # goc 通常是成员数
                                        ))
                                    else:
                                        print(f"警告: {group_type} 中的 group_info 不是字典: {group_info}")
                        return groups
                    elif response_data.get("ec") == 4 or response_data.get("ec") == 1: # 4 或 1 通常表示未登录或登录失效
                        print(f"获取群列表失败，登录状态已失效或权限不足 (ec: {response_data.get('ec')})")
                        print(f"当前Cookies: {self.session.cookies.get_dict()}")
                        return []
                    else:
                        print(f"获取群列表返回未知错误码 (ec: {response_data.get('ec')}, em: {response_data.get('em')})")
                        print(f"完整响应: {response_data}")
                        return []
                except json.JSONDecodeError as e:
                    print(f"获取群列表时JSON解析失败: {str(e)}")
                    print(f"原始响应文本: {response.text[:200]}...") # 只打印一部分
                    return []
                except Exception as e_inner: # 捕获其他在解析或处理数据时可能发生的错误
                    print(f"处理群列表数据时发生内部错误: {str(e_inner)}")
                    return []
            else:
                print(f"获取群列表请求失败，状态码: {response.status_code}")
                print(f"响应文本: {response.text[:200]}...")
            return []
        except Exception as e:
            print(f"获取群列表时发生异常: {str(e)}")
            return []

    def _get_g_tk(self) -> str:
        """
        计算g_tk/bkn值，使用skey
        :return: g_tk值
        """
        skey = self.session.cookies.get("skey")
        if not skey:
            print("警告: 计算g_tk时skey为空!")
            return "0" # 或者其他默认/错误值
            
        hash_val = 5381
        for char in skey:
            hash_val += (hash_val << 5) + ord(char)
        return str(hash_val & 2147483647)

# 保留 main 函数用于可能的独立测试，但 Web 应用不会直接调用它
def main():
    api = QQGroupAPI()
    qrsig = api.initiate_qr_and_get_qrsig()
    if qrsig:
        print(f"成功获取 qrsig: {qrsig}")
        print("现在请扫描保存在 qq_login_qr.png 的二维码。")
        # 这里可以暂停等待用户操作，或者直接进入轮询
        # input("扫描二维码后按 Enter 继续...") # 用于手动测试
        if api.poll_scan_status_and_login(qrsig):
            print("登录流程成功完成。")
            print("开始获取群列表...")
            groups = api.get_joined_groups()
            if groups:
                print(f"共加入了 {len(groups)} 个群：")
                for group in groups:
                    print(f"群名：{group.group_name} | 群号：{group.group_id} | 成员数：{group.member_count}")
            else:
                print("未能获取到群列表或未加入任何群。")
        else:
            print("登录流程失败。")
    else:
        print("初始化二维码和获取qrsig失败。")

if __name__ == "__main__":
    main() 