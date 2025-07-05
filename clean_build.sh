#!/bin/bash

echo "开始清理编译文件..."

# 清理前端编译输出
echo "清理前端编译输出..."
rm -rf web/ui
rm -rf web/dist

# 清理服务端UI文件
echo "清理服务端UI文件..."
rm -rf server/ui
rm -rf server/anylink

# 清理部署文件
echo "清理部署文件..."
rm -rf anylink-deploy
rm -rf anylink-v4.1
rm -f anylink-deploy*.tar.gz

# 清理日志
echo "清理构建日志..."
rm -rf build_logs/*.log

echo "清理完成！"
echo ""
echo "现在你可以运行以下命令重新构建："
echo "1. ./build_all.sh          # 构建前端和后端"
echo "2. ./build_all.sh -w       # 仅构建前端"
echo "3. ./build_all.sh -s       # 仅构建后端"