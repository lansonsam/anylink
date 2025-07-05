#!/bin/bash

# AnyLink 完整构建脚本
# 包含前端、后端构建，带有详细日志记录

# 设置颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 当前目录
CURRENT_DIR=$(pwd)
# 日志文件
LOG_DIR="$CURRENT_DIR/build_logs"
LOG_FILE="$LOG_DIR/build_$(date +%Y%m%d_%H%M%S).log"
# 错误计数
ERROR_COUNT=0

# 创建日志目录
mkdir -p "$LOG_DIR"

# 日志函数
log() {
    local level=$1
    shift
    local message="$@"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case $level in
        INFO)
            echo -e "${BLUE}[INFO]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ;;
        SUCCESS)
            echo -e "${GREEN}[SUCCESS]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ;;
        WARN)
            echo -e "${YELLOW}[WARN]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ;;
        ERROR)
            echo -e "${RED}[ERROR]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ((ERROR_COUNT++))
            ;;
    esac
}

# 错误处理函数
handle_error() {
    local exit_code=$1
    local error_msg=$2
    if [ $exit_code -ne 0 ]; then
        log ERROR "$error_msg (退出码: $exit_code)"
        return 1
    fi
    return 0
}

# 检查命令是否存在
check_command() {
    local cmd=$1
    if ! command -v $cmd &> /dev/null; then
        log ERROR "未找到命令: $cmd，请先安装"
        return 1
    fi
    return 0
}

# 显示使用帮助
show_help() {
    echo "使用方法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help        显示帮助信息"
    echo "  -w, --web-only    仅构建前端"
    echo "  -s, --server-only 仅构建后端"
    echo "  -d, --docker      使用 Docker 构建"
    echo "  -l, --local       本地构建（默认）"
    echo "  -c, --clean       清理构建产物"
    echo "  -v, --verbose     显示详细输出"
    echo ""
    echo "示例:"
    echo "  $0              # 完整构建（前端+后端）"
    echo "  $0 -w           # 仅构建前端"
    echo "  $0 -s           # 仅构建后端"
    echo "  $0 -d           # 使用 Docker 构建"
    echo "  $0 -c           # 清理构建产物"
}

# 清理构建产物
clean_build() {
    log INFO "开始清理构建产物..."
    
    # 清理前端
    if [ -d "web/ui" ]; then
        rm -rf web/ui
        log SUCCESS "已清理前端构建目录: web/ui"
    fi
    
    if [ -d "server/ui" ]; then
        rm -rf server/ui
        log SUCCESS "已清理服务端UI目录: server/ui"
    fi
    
    # 清理后端
    if [ -f "server/anylink" ]; then
        rm -f server/anylink
        log SUCCESS "已清理后端二进制文件: server/anylink"
    fi
    
    # 清理部署目录
    if [ -d "anylink-deploy" ]; then
        rm -rf anylink-deploy
        log SUCCESS "已清理部署目录: anylink-deploy"
    fi
    
    # 清理压缩包
    rm -f anylink-deploy*.tar.gz
    log SUCCESS "已清理压缩包"
    
    log SUCCESS "清理完成"
}

# 构建前端
build_web() {
    log INFO "开始构建前端..."
    
    # 检查 Node.js 环境
    if ! check_command node; then
        log WARN "未检测到 Node.js，尝试使用 Docker 构建..."
        build_web_docker
        return $?
    fi
    
    cd "$CURRENT_DIR/web" || {
        log ERROR "无法进入 web 目录"
        return 1
    }
    
    # 检查 package.json 是否存在
    if [ ! -f "package.json" ]; then
        log ERROR "未找到 package.json 文件"
        return 1
    fi
    
    # 安装依赖
    log INFO "安装前端依赖..."
    if command -v yarn &> /dev/null; then
        yarn install --registry=https://registry.npmmirror.com 2>&1 | tee -a "$LOG_FILE"
    else
        npm install --registry=https://registry.npmmirror.com 2>&1 | tee -a "$LOG_FILE"
    fi
    
    handle_error $? "前端依赖安装失败" || return 1
    
    # 构建前端
    log INFO "构建前端项目..."
    # 修复 Node.js 17+ 与旧版 Webpack 的兼容性问题
    export NODE_OPTIONS=--openssl-legacy-provider
    
    if command -v yarn &> /dev/null; then
        yarn run build 2>&1 | tee -a "$LOG_FILE"
    else
        npm run build 2>&1 | tee -a "$LOG_FILE"
    fi
    
    handle_error $? "前端构建失败" || return 1
    
    # 复制前端文件到服务端
    log INFO "复制前端文件到服务端..."
    cd "$CURRENT_DIR"
    
    # 先清理旧的 UI 目录
    rm -rf server/ui
    
    # 复制新的 UI 文件
    cp -r web/ui server/ui
    
    handle_error $? "复制前端文件失败" || return 1
    
    log SUCCESS "前端构建完成"
    return 0
}

# 使用 Docker 构建前端
build_web_docker() {
    log INFO "使用 Docker 构建前端..."
    
    cd "$CURRENT_DIR" || return 1
    
    # 清理旧文件
    rm -rf web/ui server/ui
    
    # 使用 Docker 构建
    docker run -it --rm -v "$PWD/web:/app" -w /app node:16-alpine \
        sh -c "yarn install --registry=https://registry.npmmirror.com && yarn run build" 2>&1 | tee -a "$LOG_FILE"
    
    handle_error $? "Docker 前端构建失败" || return 1
    
    # 复制前端文件
    cp -r web/ui server/ui
    
    handle_error $? "复制前端文件失败" || return 1
    
    log SUCCESS "Docker 前端构建完成"
    return 0
}

# 构建后端
build_server() {
    log INFO "开始构建后端..."
    
    # 获取版本信息
    if [ ! -f "version" ]; then
        log ERROR "未找到 version 文件"
        return 1
    fi
    
    VERSION=$(cat version)
    COMMIT_ID=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    BUILD_DATE=$(date -Iseconds)
    
    log INFO "版本: $VERSION"
    log INFO "提交ID: $COMMIT_ID"
    log INFO "构建时间: $BUILD_DATE"
    
    # 检查 UI 文件
    if [ ! -d "server/ui" ]; then
        log WARN "未找到前端文件，需要先构建前端"
        build_web || return 1
    fi
    
    # 检查 Go 环境
    if ! check_command go; then
        log ERROR "未检测到 Go 环境"
        return 1
    fi
    
    cd "$CURRENT_DIR/server" || {
        log ERROR "无法进入 server 目录"
        return 1
    }
    
    # 设置 Go 环境变量
    export GO111MODULE=on
    export GOPROXY=https://goproxy.cn,direct
    
    # 下载依赖
    log INFO "下载 Go 依赖..."
    go mod download 2>&1 | tee -a "$LOG_FILE"
    
    handle_error $? "Go 依赖下载失败" || return 1
    
    # 构建二进制文件
    log INFO "构建后端二进制文件..."
    go build -v -trimpath \
        -ldflags "-s -w -X main.appVer=$VERSION -X main.commitId=$COMMIT_ID -X main.buildDate=$BUILD_DATE" \
        -o anylink main.go 2>&1 | tee -a "$LOG_FILE"
    
    handle_error $? "后端构建失败" || return 1
    
    # 验证构建结果
    if [ -f "anylink" ]; then
        log SUCCESS "后端构建成功"
        ./anylink -v
    else
        log ERROR "未找到构建的二进制文件"
        return 1
    fi
    
    cd "$CURRENT_DIR"
    return 0
}

# 使用 Docker 构建
build_docker() {
    log INFO "开始 Docker 构建..."
    
    VERSION=$(cat version)
    
    # 构建前端（如果需要）
    if [ ! -d "server/ui" ]; then
        build_web_docker || return 1
    fi
    
    # 构建 Docker 镜像
    log INFO "构建 Docker 镜像..."
    docker build -t bjdgyc/anylink:latest --no-cache --progress=plain \
        --build-arg CN="yes" --build-arg appVer=$VERSION --build-arg commitId=$(git rev-parse HEAD) \
        -f docker/Dockerfile . 2>&1 | tee -a "$LOG_FILE"
    
    handle_error $? "Docker 镜像构建失败" || return 1
    
    # 标记版本
    docker tag bjdgyc/anylink:latest bjdgyc/anylink:$VERSION
    
    log SUCCESS "Docker 构建完成"
    log INFO "镜像: bjdgyc/anylink:$VERSION"
    return 0
}

# 创建部署包
create_deploy_package() {
    log INFO "创建部署包..."
    
    VERSION=$(cat version)
    DEPLOY_DIR="anylink-deploy"
    DEPLOY_ARCHIVE="anylink-deploy-$VERSION.tar.gz"
    
    # 清理旧的部署目录
    rm -rf "$DEPLOY_DIR"
    mkdir -p "$DEPLOY_DIR"
    
    # 复制必要文件
    log INFO "复制文件..."
    cp server/anylink "$DEPLOY_DIR/" || {
        log ERROR "未找到 anylink 二进制文件"
        return 1
    }
    
    # 复制配置文件
    mkdir -p "$DEPLOY_DIR/conf"
    cp server/conf/server-sample.toml "$DEPLOY_DIR/conf/server.toml"
    cp server/conf/profile.xml "$DEPLOY_DIR/conf/" || {
        log WARN "profile.xml 不存在，创建默认配置"
        # 创建默认 profile.xml
        cat > "$DEPLOY_DIR/conf/profile.xml" << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<AnyConnectProfile xmlns="http://schemas.xmlsoap.org/encoding/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://schemas.xmlsoap.org/encoding/ AnyConnectProfile.xsd">
    <ClientInitialization>
        <UseStartBeforeLogon UserControllable="false">false</UseStartBeforeLogon>
        <StrictCertificateTrust>false</StrictCertificateTrust>
        <RestrictPreferenceCaching>false</RestrictPreferenceCaching>
        <RestrictTunnelProtocols>IPSec</RestrictTunnelProtocols>
        <BypassDownloader>true</BypassDownloader>
        <WindowsVPNEstablishment>AllowRemoteUsers</WindowsVPNEstablishment>
        <CertEnrollmentPin>pinAllowed</CertEnrollmentPin>
        <CertificateMatch>
            <KeyUsage>
                <MatchKey>Digital_Signature</MatchKey>
            </KeyUsage>
            <ExtendedKeyUsage>
                <ExtendedMatchKey>ClientAuth</ExtendedMatchKey>
            </ExtendedKeyUsage>
        </CertificateMatch>
    </ClientInitialization>

    <ServerList>
        <HostEntry>
            <HostName>VPN Server</HostName>
            <HostAddress>localhost</HostAddress>
        </HostEntry>
    </ServerList>
</AnyConnectProfile>
EOF
    }
    
    # 复制证书文件（如果存在）
    if [ -f "server/conf/vpn_cert.pem" ] && [ -f "server/conf/vpn_cert.key" ]; then
        cp server/conf/vpn_cert.pem server/conf/vpn_cert.key "$DEPLOY_DIR/conf/"
    else
        log WARN "证书文件不存在，需要手动配置证书"
    fi
    
    cp -r server/conf/files "$DEPLOY_DIR/conf/"
    
    # 复制部署文件
    mkdir -p "$DEPLOY_DIR/deploy"
    cp -r deploy/* "$DEPLOY_DIR/deploy/"
    
    # 复制文档
    cp README.md LICENSE "$DEPLOY_DIR/"
    
    # 创建启动脚本
    cat > "$DEPLOY_DIR/start.sh" << 'EOF'
#!/bin/bash
./anylink -c ./conf/server.toml
EOF
    chmod +x "$DEPLOY_DIR/start.sh"
    
    # 创建压缩包
    log INFO "创建压缩包..."
    tar zcf "$DEPLOY_ARCHIVE" "$DEPLOY_DIR"
    
    handle_error $? "创建压缩包失败" || return 1
    
    # 显示部署包信息
    log SUCCESS "部署包创建完成"
    log INFO "部署目录: $DEPLOY_DIR"
    log INFO "压缩包: $DEPLOY_ARCHIVE"
    ls -lh "$DEPLOY_DIR"
    
    return 0
}

# 完整构建
build_all() {
    log INFO "开始完整构建..."
    
    # 构建前端
    build_web || {
        log ERROR "前端构建失败，停止构建"
        return 1
    }
    
    # 构建后端
    build_server || {
        log ERROR "后端构建失败，停止构建"
        return 1
    }
    
    # 创建部署包
    create_deploy_package || {
        log ERROR "创建部署包失败"
        return 1
    }
    
    log SUCCESS "完整构建成功"
    return 0
}

# 主函数
main() {
    log INFO "=== AnyLink 构建开始 ==="
    log INFO "日志文件: $LOG_FILE"
    
    # 解析命令行参数
    BUILD_WEB=false
    BUILD_SERVER=false
    BUILD_DOCKER=false
    CLEAN_ONLY=false
    
    if [ $# -eq 0 ]; then
        # 无参数，执行完整构建
        build_all
    else
        while [[ $# -gt 0 ]]; do
            case $1 in
                -h|--help)
                    show_help
                    exit 0
                    ;;
                -w|--web-only)
                    BUILD_WEB=true
                    shift
                    ;;
                -s|--server-only)
                    BUILD_SERVER=true
                    shift
                    ;;
                -d|--docker)
                    BUILD_DOCKER=true
                    shift
                    ;;
                -c|--clean)
                    CLEAN_ONLY=true
                    shift
                    ;;
                -v|--verbose)
                    set -x
                    shift
                    ;;
                *)
                    log ERROR "未知选项: $1"
                    show_help
                    exit 1
                    ;;
            esac
        done
        
        # 执行构建
        if [ "$CLEAN_ONLY" = true ]; then
            clean_build
        elif [ "$BUILD_DOCKER" = true ]; then
            build_docker
        elif [ "$BUILD_WEB" = true ] && [ "$BUILD_SERVER" = false ]; then
            build_web
        elif [ "$BUILD_SERVER" = true ] && [ "$BUILD_WEB" = false ]; then
            build_server
            create_deploy_package
        else
            build_all
        fi
    fi
    
    # 显示构建结果
    log INFO "=== 构建结束 ==="
    if [ $ERROR_COUNT -gt 0 ]; then
        log ERROR "构建过程中出现 $ERROR_COUNT 个错误"
        exit 1
    else
        log SUCCESS "构建成功完成"
        log INFO "日志文件保存在: $LOG_FILE"
    fi
}

# 执行主函数
main "$@"