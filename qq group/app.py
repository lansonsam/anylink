from flask import Flask, request, jsonify
from qq_group_api import QQGroupAPI
import jwt
import secrets
import threading
import time
import sqlite3
import os
import hashlib
from datetime import datetime, timedelta

app = Flask(__name__)
app.secret_key = secrets.token_hex(16)
JWT_SECRET = secrets.token_hex(32)

# 牛子公益API
API_NAME = "牛子公益API"
DATABASE_FILE = "qq_bindings.db"

# 存储QQ号和登录状态的字典
login_sessions = {}

def init_database():
    """初始化数据库表"""
    conn = sqlite3.connect(DATABASE_FILE)
    cursor = conn.cursor()
    
    # 创建QQ绑定表
    cursor.execute('''
        CREATE TABLE IF NOT EXISTS qq_bindings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            qq_number TEXT UNIQUE NOT NULL,
            card_key TEXT NOT NULL,
            verification_value INTEGER DEFAULT 1,
            bind_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            last_update TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    ''')
    
    # 创建验证令牌表
    cursor.execute('''
        CREATE TABLE IF NOT EXISTS verification_tokens (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            qq_number TEXT NOT NULL,
            token TEXT UNIQUE NOT NULL,
            expires_at TIMESTAMP NOT NULL,
            used INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    ''')
    
    conn.commit()
    conn.close()
    print(f"[{API_NAME}] 数据库初始化完成")

def generate_verification_token(qq_number: str) -> str:
    """生成验证令牌"""
    # 生成唯一令牌
    timestamp = str(int(time.time()))
    random_str = secrets.token_hex(16)
    token_data = f"{qq_number}-{timestamp}-{random_str}"
    token = hashlib.sha256(token_data.encode()).hexdigest()
    
    # 存储到数据库，有效期30分钟
    conn = sqlite3.connect(DATABASE_FILE)
    cursor = conn.cursor()
    
    expires_at = datetime.now() + timedelta(minutes=30)
    cursor.execute('''
        INSERT INTO verification_tokens (qq_number, token, expires_at)
        VALUES (?, ?, ?)
    ''', (qq_number, token, expires_at))
    
    conn.commit()
    conn.close()
    
    return token

def verify_token(token: str) -> tuple:
    """验证令牌有效性，返回(is_valid, qq_number)"""
    conn = sqlite3.connect(DATABASE_FILE)
    cursor = conn.cursor()
    
    cursor.execute('''
        SELECT qq_number, expires_at, used FROM verification_tokens 
        WHERE token = ?
    ''', (token,))
    
    result = cursor.fetchone()
    
    if not result:
        conn.close()
        return False, None
        
    qq_number, expires_at_str, used = result
    
    # 检查是否已使用
    if used:
        conn.close()
        return False, "Token已被使用"
    
    # 检查是否过期
    expires_at = datetime.fromisoformat(expires_at_str.replace(' ', 'T'))
    if datetime.now() > expires_at:
        conn.close()
        return False, "Token已过期"
    
    conn.close()
    return True, qq_number

def mark_token_used(token: str):
    """标记令牌为已使用"""
    conn = sqlite3.connect(DATABASE_FILE)
    cursor = conn.cursor()
    
    cursor.execute('''
        UPDATE verification_tokens SET used = 1 WHERE token = ?
    ''', (token,))
    
    conn.commit()
    conn.close()

def process_qq_login_verification(qq_number_str: str):
    """
    后台线程任务：
    1. 等待用户扫描二维码并登录QQ
    2. 验证QQ号真实性（仅验证能成功登录即可）
    3. 登录成功后生成验证令牌
    """
    session_data = login_sessions.get(qq_number_str)
    if not session_data or "api_instance" not in session_data or "qrsig" not in session_data:
        print(f"[Thread-{qq_number_str}] 无效的会话数据，终止线程")
        if qq_number_str in login_sessions:
            del login_sessions[qq_number_str]
        return

    api = session_data["api_instance"]
    qrsig = session_data["qrsig"]

    print(f"[Thread-{qq_number_str}] 开始轮询二维码扫描状态...")
    login_successful = api.poll_scan_status_and_login(qrsig)

    if not login_successful:
        print(f"[Thread-{qq_number_str}] QQ登录失败或二维码失效")
        login_sessions[qq_number_str]["status"] = "login_failed"
        return

    print(f"[Thread-{qq_number_str}] QQ登录成功，验证QQ号真实性通过")
    
    # QQ登录成功即表示QQ号验证通过，生成验证令牌
    try:
        token = generate_verification_token(qq_number_str)
        login_sessions[qq_number_str]["status"] = "qq_verified_success"
        login_sessions[qq_number_str]["verification_token"] = token
        login_sessions[qq_number_str]["verified_time"] = time.time()
        print(f"[Thread-{qq_number_str}] 验证令牌生成成功")
    except Exception as e:
        print(f"[Thread-{qq_number_str}] 验证令牌生成失败: {e}")
        login_sessions[qq_number_str]["status"] = "token_generation_failed"

@app.route('/')
def index():
    return jsonify({
        "api_name": API_NAME,
        "version": "2.0",
        "description": "QQ验证令牌绑定卡密系统API",
        "endpoints": {
            "/api/qq/verify": "开始QQ号验证（生成二维码）",
            "/api/qq/status": "查询QQ验证状态",
            "/api/qq/bind": "使用验证令牌绑定卡密",
            "/api/qq/query": "查询QQ绑定状态",
            "/qq_login_qr.png": "获取二维码图片"
        },
        "flow": [
            "1. 调用 /api/qq/verify 开始验证",
            "2. 扫描二维码登录QQ",
            "3. 调用 /api/qq/status 获取验证令牌",
            "4. 使用令牌调用 /api/qq/bind 绑定卡密"
        ]
    })

@app.route('/qq_login_qr.png')
def get_qr():
    qr_path = "qq_login_qr.png"
    if os.path.exists(qr_path):
        from flask import send_file
        return send_file(qr_path, mimetype='image/png')
    else:
        return jsonify({"error": "二维码未生成"}), 404

@app.route('/api/qq/verify', methods=['POST'])
def qq_verify_start():
    """开始QQ号验证流程"""
    data = request.get_json()
    if not data or 'qq_number' not in data:
        return jsonify({"error": "请提供QQ号", "code": 400}), 400
    
    qq_number = str(data['qq_number'])
    
    # 检查QQ号格式（简单验证）
    if not qq_number.isdigit() or len(qq_number) < 5:
        return jsonify({"error": "QQ号格式无效", "code": 400}), 400
    
    # 检查是否有进行中的会话
    if qq_number in login_sessions and login_sessions[qq_number].get("status") not in [
        "login_failed", "qq_verified_success", "token_generation_failed"
    ]:
        return jsonify({
            "error": "该QQ号已有进行中的验证会话", 
            "code": 409,
            "current_status": login_sessions[qq_number].get('status')
        }), 409

    try:
        api = QQGroupAPI()
        print(f"[{API_NAME}] 为QQ号 {qq_number} 初始化二维码生成...")
        qrsig = api.initiate_qr_and_get_qrsig()

        if qrsig:
            login_sessions[qq_number] = {
                "status": "waiting_qr_scan", 
                "qrsig": qrsig,
                "api_instance": api,
                "timestamp": time.time()
            }
            
            thread = threading.Thread(target=process_qq_login_verification, args=(qq_number,))
            thread.daemon = True
            thread.start()
            
            return jsonify({
                "success": True,
                "message": "二维码已生成，请使用QQ扫描登录验证QQ号真实性",
                "qq_number": qq_number,
                "qr_url": "/qq_login_qr.png",
                "code": 200
            })
        else:
            return jsonify({"error": "二维码生成失败", "code": 500}), 500
            
    except Exception as e:
        print(f"[{API_NAME}] QQ验证启动失败: {e}")
        return jsonify({"error": "验证启动失败", "code": 500}), 500

@app.route('/api/qq/status', methods=['POST'])
def qq_verify_status():
    """查询QQ验证状态，成功后返回验证令牌"""
    data = request.get_json()
    if not data or 'qq_number' not in data:
        return jsonify({"error": "请提供QQ号", "code": 400}), 400

    qq_number = str(data['qq_number'])
    session_data = login_sessions.get(qq_number)

    if not session_data:
        return jsonify({"error": "无效的验证会话", "code": 404}), 404

    current_status = session_data.get("status")
    
    status_messages = {
        "waiting_qr_scan": "等待扫描二维码",
        "qr_scan_success_polling_groups": "扫码成功，正在验证QQ号",
        "qq_verified_success": "QQ号验证成功",
        "login_failed": "QQ登录失败或二维码已失效",
        "token_generation_failed": "验证令牌生成失败"
    }
    
    if current_status == "qq_verified_success":
        verification_token = session_data.get("verification_token")
        verified_time = session_data.get("verified_time")
        
        # 清理验证会话（令牌已生成）
        if qq_number in login_sessions:
            del login_sessions[qq_number]
        
        return jsonify({
            "success": True,
            "status": "verified",
            "message": status_messages.get(current_status),
            "qq_number": qq_number,
            "verification_token": verification_token,
            "verified_time": verified_time,
            "token_expires_minutes": 30,
            "code": 200
        })
    elif current_status in ["login_failed", "token_generation_failed"]:
        # 清理失败的会话
        if qq_number in login_sessions:
            del login_sessions[qq_number]
        return jsonify({
            "success": False,
            "status": "failed",
            "message": status_messages.get(current_status),
            "code": 401
        }), 401
    else:
        return jsonify({
            "success": True,
            "status": "pending",
            "message": status_messages.get(current_status, "处理中"),
            "code": 200
        })

@app.route('/api/qq/bind', methods=['POST'])
def qq_bind_card():
    """使用验证令牌绑定卡密"""
    data = request.get_json()
    if not data or 'verification_token' not in data or 'card_key' not in data:
        return jsonify({"error": "请提供验证令牌和卡密", "code": 400}), 400
    
    verification_token = str(data['verification_token'])
    card_key = str(data['card_key'])
    
    # 验证令牌
    is_valid, qq_number_or_error = verify_token(verification_token)
    if not is_valid:
        error_msg = qq_number_or_error if isinstance(qq_number_or_error, str) else "无效的验证令牌"
        return jsonify({"error": error_msg, "code": 403}), 403
    
    qq_number = qq_number_or_error
    
    try:
        conn = sqlite3.connect(DATABASE_FILE)
        cursor = conn.cursor()
        
        # 检查QQ是否已绑定
        cursor.execute("SELECT card_key FROM qq_bindings WHERE qq_number = ?", (qq_number,))
        existing = cursor.fetchone()
        
        if existing:
            # 更新绑定
            cursor.execute("""
                UPDATE qq_bindings 
                SET card_key = ?, last_update = CURRENT_TIMESTAMP, verification_value = 1
                WHERE qq_number = ?
            """, (card_key, qq_number))
            action = "updated"
        else:
            # 新建绑定
            cursor.execute("""
                INSERT INTO qq_bindings (qq_number, card_key, verification_value) 
                VALUES (?, ?, 1)
            """, (qq_number, card_key))
            action = "created"
        
        conn.commit()
        conn.close()
        
        # 标记令牌为已使用
        mark_token_used(verification_token)
        
        print(f"[{API_NAME}] QQ {qq_number} 绑定卡密成功: {card_key}")
        
        return jsonify({
            "success": True,
            "message": f"卡密绑定{action}成功",
            "qq_number": qq_number,
            "card_key": card_key,
            "bind_time": datetime.now().isoformat(),
            "code": 200
        })
        
    except sqlite3.Error as e:
        print(f"[{API_NAME}] 数据库操作失败: {e}")
        return jsonify({"error": "绑定失败，数据库错误", "code": 500}), 500
    except Exception as e:
        print(f"[{API_NAME}] 绑定操作失败: {e}")
        return jsonify({"error": "绑定失败", "code": 500}), 500

@app.route('/api/qq/query', methods=['POST'])
def qq_query_binding():
    """查询QQ绑定状态"""
    data = request.get_json()
    if not data or 'qq_number' not in data:
        return jsonify({"error": "请提供QQ号", "code": 400}), 400
    
    qq_number = str(data['qq_number'])
    
    try:
        conn = sqlite3.connect(DATABASE_FILE)
        cursor = conn.cursor()
        
        cursor.execute("""
            SELECT qq_number, card_key, verification_value, bind_time, last_update 
            FROM qq_bindings 
            WHERE qq_number = ?
        """, (qq_number,))
        
        result = cursor.fetchone()
        conn.close()
        
        if result:
            return jsonify({
                "success": True,
                "bound": True,
                "data": {
                    "qq_number": result[0],
                    "card_key": result[1],
                    "verification_value": result[2],
                    "bind_time": result[3],
                    "last_update": result[4]
                },
                "code": 200
            })
        else:
            return jsonify({
                "success": True,
                "bound": False,
                "message": "该QQ号未绑定卡密",
                "code": 200
            })
            
    except sqlite3.Error as e:
        print(f"[{API_NAME}] 查询数据库失败: {e}")
        return jsonify({"error": "查询失败，数据库错误", "code": 500}), 500
    except Exception as e:
        print(f"[{API_NAME}] 查询操作失败: {e}")
        return jsonify({"error": "查询失败", "code": 500}), 500

if __name__ == '__main__':
    # 初始化数据库
    init_database()
    print(f"[{API_NAME}] 服务启动成功")
    print(f"API文档: http://localhost:5000/")
    print(f"验证流程: QQ号验证 -> 获取令牌 -> 绑定卡密")
    app.run(debug=True, port=5000, threaded=True) 