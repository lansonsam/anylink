#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
牛子公益API - 远程服务器
接收本地QQ验证凭证，处理卡密绑定
"""

from flask import Flask, request, jsonify
import sqlite3
import os
import hashlib
import time
from datetime import datetime

app = Flask(__name__)

# 牛子公益API - 远程服务器配置
API_NAME = "牛子公益API - 远程服务器"
DATABASE_FILE = "remote_qq_bindings.db"
VERIFICATION_SECRET = "your_secret_key_here"  # 修改为您的密钥

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
            verification_proof TEXT NOT NULL,
            verified_time TEXT NOT NULL,
            verifier_type TEXT DEFAULT 'local_qq_verifier',
            bind_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            last_update TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    ''')
    
    # 创建验证日志表
    cursor.execute('''
        CREATE TABLE IF NOT EXISTS verification_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            qq_number TEXT NOT NULL,
            action TEXT NOT NULL,
            result TEXT NOT NULL,
            client_ip TEXT,
            timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    ''')
    
    conn.commit()
    conn.close()
    print(f"[{API_NAME}] 数据库初始化完成")

def log_verification(qq_number: str, action: str, result: str, client_ip: str = None):
    """记录验证日志"""
    try:
        conn = sqlite3.connect(DATABASE_FILE)
        cursor = conn.cursor()
        
        cursor.execute('''
            INSERT INTO verification_logs (qq_number, action, result, client_ip)
            VALUES (?, ?, ?, ?)
        ''', (qq_number, action, result, client_ip))
        
        conn.commit()
        conn.close()
    except Exception as e:
        print(f"日志记录失败: {e}")

def validate_verification_proof(qq_number: str, proof: str) -> bool:
    """验证本地验证凭证的有效性"""
    # 这里可以添加更复杂的验证逻辑
    # 基本检查：确保是64位十六进制字符串
    if not proof or len(proof) != 64:
        return False
    
    try:
        # 验证是否为有效的十六进制
        int(proof, 16)
        return True
    except ValueError:
        return False

@app.route('/')
def index():
    return jsonify({
        "api_name": API_NAME,
        "version": "1.0",
        "description": "远程QQ绑定卡密API服务器",
        "endpoints": {
            "/api/bind_with_proof": "使用本地验证凭证绑定卡密",
            "/api/query_binding": "查询QQ绑定状态",
            "/api/stats": "获取服务统计信息"
        },
        "supported_verifiers": [
            "local_qq_verifier"
        ]
    })

@app.route('/api/bind_with_proof', methods=['POST'])
def bind_with_proof():
    """使用本地验证凭证绑定卡密"""
    client_ip = request.remote_addr
    
    try:
        data = request.get_json()
        if not data:
            return jsonify({"error": "请求数据为空", "code": 400}), 400
        
        # 验证必需参数
        required_fields = ['qq_number', 'card_key', 'verification_proof']
        for field in required_fields:
            if field not in data:
                log_verification("unknown", "bind_attempt", f"缺少字段: {field}", client_ip)
                return jsonify({"error": f"缺少必需字段: {field}", "code": 400}), 400
        
        qq_number = str(data['qq_number'])
        card_key = str(data['card_key'])
        verification_proof = str(data['verification_proof'])
        verified_time = data.get('verified_time', datetime.now().isoformat())
        verifier_type = data.get('verifier_type', 'local_qq_verifier')
        
        # 验证QQ号格式
        if not qq_number.isdigit() or len(qq_number) < 5:
            log_verification(qq_number, "bind_attempt", "QQ号格式无效", client_ip)
            return jsonify({"error": "QQ号格式无效", "code": 400}), 400
        
        # 验证凭证有效性
        if not validate_verification_proof(qq_number, verification_proof):
            log_verification(qq_number, "bind_attempt", "验证凭证无效", client_ip)
            return jsonify({"error": "验证凭证无效", "code": 403}), 403
        
        # 数据库操作
        conn = sqlite3.connect(DATABASE_FILE)
        cursor = conn.cursor()
        
        # 检查是否已绑定
        cursor.execute("SELECT card_key, bind_time FROM qq_bindings WHERE qq_number = ?", (qq_number,))
        existing = cursor.fetchone()
        
        if existing:
            # 更新绑定
            cursor.execute('''
                UPDATE qq_bindings 
                SET card_key = ?, verification_proof = ?, verified_time = ?, 
                    verifier_type = ?, last_update = CURRENT_TIMESTAMP
                WHERE qq_number = ?
            ''', (card_key, verification_proof, verified_time, verifier_type, qq_number))
            
            action = "updated"
            message = "卡密绑定更新成功"
        else:
            # 新建绑定
            cursor.execute('''
                INSERT INTO qq_bindings 
                (qq_number, card_key, verification_proof, verified_time, verifier_type)
                VALUES (?, ?, ?, ?, ?)
            ''', (qq_number, card_key, verification_proof, verified_time, verifier_type))
            
            action = "created"
            message = "卡密绑定创建成功"
        
        conn.commit()
        conn.close()
        
        # 记录成功日志
        log_verification(qq_number, "bind_success", f"卡密绑定{action}", client_ip)
        
        print(f"[{API_NAME}] QQ {qq_number} 绑定卡密成功: {card_key} (来自 {client_ip})")
        
        return jsonify({
            "success": True,
            "message": message,
            "qq_number": qq_number,
            "card_key": card_key,
            "action": action,
            "bind_time": datetime.now().isoformat(),
            "code": 200
        })
        
    except sqlite3.Error as e:
        log_verification(data.get('qq_number', 'unknown'), "bind_error", f"数据库错误: {str(e)}", client_ip)
        print(f"[{API_NAME}] 数据库操作失败: {e}")
        return jsonify({"error": "数据库操作失败", "code": 500}), 500
    except Exception as e:
        log_verification(data.get('qq_number', 'unknown'), "bind_error", f"系统错误: {str(e)}", client_ip)
        print(f"[{API_NAME}] 绑定操作失败: {e}")
        return jsonify({"error": "绑定操作失败", "code": 500}), 500

@app.route('/api/query_binding', methods=['POST'])
def query_binding():
    """查询QQ绑定状态"""
    client_ip = request.remote_addr
    
    try:
        data = request.get_json()
        if not data or 'qq_number' not in data:
            return jsonify({"error": "请提供QQ号", "code": 400}), 400
        
        qq_number = str(data['qq_number'])
        
        conn = sqlite3.connect(DATABASE_FILE)
        cursor = conn.cursor()
        
        cursor.execute('''
            SELECT qq_number, card_key, verified_time, verifier_type, 
                   bind_time, last_update
            FROM qq_bindings 
            WHERE qq_number = ?
        ''', (qq_number,))
        
        result = cursor.fetchone()
        conn.close()
        
        if result:
            log_verification(qq_number, "query_success", "查询成功", client_ip)
            
            return jsonify({
                "success": True,
                "bound": True,
                "data": {
                    "qq_number": result[0],
                    "card_key": result[1],
                    "verified_time": result[2],
                    "verifier_type": result[3],
                    "bind_time": result[4],
                    "last_update": result[5]
                },
                "code": 200
            })
        else:
            log_verification(qq_number, "query_success", "未绑定", client_ip)
            
            return jsonify({
                "success": True,
                "bound": False,
                "message": "该QQ号未绑定卡密",
                "code": 200
            })
            
    except Exception as e:
        print(f"[{API_NAME}] 查询操作失败: {e}")
        return jsonify({"error": "查询失败", "code": 500}), 500

@app.route('/api/stats', methods=['GET'])
def get_stats():
    """获取服务统计信息"""
    try:
        conn = sqlite3.connect(DATABASE_FILE)
        cursor = conn.cursor()
        
        # 统计绑定数量
        cursor.execute("SELECT COUNT(*) FROM qq_bindings")
        total_bindings = cursor.fetchone()[0]
        
        # 统计今日操作
        today = datetime.now().date().isoformat()
        cursor.execute("SELECT COUNT(*) FROM verification_logs WHERE DATE(timestamp) = ?", (today,))
        today_operations = cursor.fetchone()[0]
        
        # 统计各类操作
        cursor.execute('''
            SELECT action, COUNT(*) 
            FROM verification_logs 
            GROUP BY action
        ''')
        action_stats = dict(cursor.fetchall())
        
        conn.close()
        
        return jsonify({
            "api_name": API_NAME,
            "total_bindings": total_bindings,
            "today_operations": today_operations,
            "action_statistics": action_stats,
            "database_file": DATABASE_FILE,
            "uptime": "运行中"
        })
        
    except Exception as e:
        return jsonify({"error": f"统计获取失败: {str(e)}"})

# 错误处理
@app.errorhandler(404)
def not_found(error):
    return jsonify({"error": "接口不存在", "code": 404}), 404

@app.errorhandler(405)
def method_not_allowed(error):
    return jsonify({"error": "请求方法不允许", "code": 405}), 405

@app.errorhandler(500)
def internal_error(error):
    return jsonify({"error": "服务器内部错误", "code": 500}), 500

if __name__ == '__main__':
    # 初始化数据库
    init_database()
    
    print(f"[{API_NAME}] 远程服务启动成功")
    print("=" * 50)
    print("🌟 牛子公益API - 远程服务器")
    print("📡 接收本地QQ验证，处理卡密绑定")
    print("=" * 50)
    print(f"📊 API文档: http://localhost:5000/")
    print(f"💾 数据库文件: {DATABASE_FILE}")
    print("=" * 50)
    
    # 启动服务器
    app.run(
        host='0.0.0.0',  # 允许外部访问
        port=5000,
        debug=True,
        threaded=True
    ) 