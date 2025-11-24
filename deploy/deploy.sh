#!/bin/bash

# 阿里云可观测MCP服务器打包和部署脚本
# 用途：构建pip包并发布到PyPI

set -e  # 遇到错误立即退出

# 脚本配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
DIST_DIR="$PROJECT_ROOT/dist"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 显示帮助信息
show_help() {
    cat << EOF
阿里云可观测MCP服务器部署脚本

用法: $0 [选项] [命令]

命令:
    build       仅构建包
    upload      仅上传包（需要先构建）
    deploy      构建并上传包（默认）
    clean       清理构建文件
    check       检查包和依赖
    test        运行测试
    help        显示此帮助信息

选项:
    --test-pypi     上传到测试PyPI服务器
    --dry-run       模拟运行，不实际执行上传
    --skip-tests    跳过测试
    --skip-checks   跳过检查
    --version       显示当前版本

示例:
    $0                    # 构建并发布到正式PyPI
    $0 --test-pypi        # 构建并发布到测试PyPI
    $0 build              # 仅构建包
    $0 clean              # 清理构建文件
    $0 check              # 检查包状态
    $0 test               # 运行测试

EOF
}

# 获取当前版本
get_version() {
    python -c "import toml; print(toml.load('pyproject.toml')['project']['version'])"
}

# 检查依赖
check_dependencies() {
    log_info "检查构建依赖..."
    
    # 检查Python版本
    python_version=$(python --version 2>&1 | cut -d' ' -f2)
    log_info "Python版本: $python_version"
    
    # 检查必要的包
    required_packages=("build" "twine" "toml")
    missing_packages=()
    
    for package in "${required_packages[@]}"; do
        if ! python -c "import $package" 2>/dev/null; then
            missing_packages+=("$package")
        fi
    done
    
    if [ ${#missing_packages[@]} -gt 0 ]; then
        log_warning "缺少必要的包: ${missing_packages[*]}"
        log_info "正在安装缺少的包..."
        pip install "${missing_packages[@]}"
    fi
    
    log_success "依赖检查完成"
}

# 运行测试
run_tests() {
    log_info "运行测试套件..."
    
    if [ -f "pytest.ini" ] || [ -f "pyproject.toml" ]; then
        if command -v pytest &> /dev/null; then
            pytest -v
        else
            log_warning "pytest未安装，跳过测试"
            return 0
        fi
    else
        log_warning "未找到测试配置，跳过测试"
        return 0
    fi
    
    log_success "测试通过"
}

# 进行包检查
check_package() {
    log_info "检查包配置和结构..."
    
    # 检查pyproject.toml
    if [ ! -f "pyproject.toml" ]; then
        log_error "未找到pyproject.toml文件"
        exit 1
    fi
    
    # 检查源码目录
    if [ ! -d "src/mcp_server_aliyun_observability" ]; then
        log_error "未找到源码目录: src/mcp_server_aliyun_observability"
        exit 1
    fi
    
    # 检查README
    if [ ! -f "README.md" ]; then
        log_warning "未找到README.md文件"
    fi
    
    # 检查版本号
    current_version=$(get_version)
    log_info "当前版本: $current_version"
    
    # 检查Git状态
    if git status --porcelain | grep -q .; then
        log_warning "工作目录有未提交的更改"
        git status --porcelain
    fi
    
    log_success "包检查完成"
}

# 清理构建文件
clean_build() {
    log_info "清理构建文件..."
    
    # 清理目录
    rm -rf "$DIST_DIR"
    rm -rf build/
    rm -rf src/*.egg-info/
    rm -rf *.egg-info/
    
    # 清理Python缓存
    find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
    find . -name "*.pyc" -delete
    find . -name "*.pyo" -delete
    
    log_success "清理完成"
}

# 构建包
build_package() {
    log_info "开始构建包..."
    
    # 确保dist目录存在
    mkdir -p "$DIST_DIR"
    
    # 使用python -m build构建
    log_info "构建wheel和源码包..."
    python -m build --wheel --sdist --outdir "$DIST_DIR"
    
    # 显示构建结果
    log_info "构建完成，生成文件："
    ls -la "$DIST_DIR"
    
    # 检查构建的包
    log_info "检查构建的包..."
    python -m twine check "$DIST_DIR"/*
    
    log_success "包构建完成"
}

# 上传包
upload_package() {
    local repository="pypi"
    local repository_url=""
    
    if [ "$USE_TEST_PYPI" = "true" ]; then
        repository="testpypi"
        repository_url="--repository-url https://test.pypi.org/legacy/"
        log_info "上传到测试PyPI服务器..."
    else
        log_info "上传到正式PyPI服务器..."
    fi
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "模拟上传模式，不会实际上传"
        log_info "将要上传的文件："
        ls -la "$DIST_DIR"
        return 0
    fi
    
    # 检查认证
    if [ "$USE_TEST_PYPI" = "true" ]; then
        log_info "使用测试PyPI，请确保已配置TEST_PYPI_TOKEN环境变量"
        if [ -z "$TEST_PYPI_TOKEN" ]; then
            log_error "请设置TEST_PYPI_TOKEN环境变量"
            exit 1
        fi
        python -m twine upload --repository testpypi "$DIST_DIR"/* --username __token__ --password "$TEST_PYPI_TOKEN"
    else
        log_info "使用正式PyPI，请确保已配置PYPI_TOKEN环境变量"
        if [ -z "$PYPI_TOKEN" ]; then
            log_error "请设置PYPI_TOKEN环境变量"
            exit 1
        fi
        python -m twine upload "$DIST_DIR"/* --username __token__ --password "$PYPI_TOKEN"
    fi
    
    log_success "包上传完成"
    
    # 显示安装命令
    local version=$(get_version)
    if [ "$USE_TEST_PYPI" = "true" ]; then
        log_info "测试安装命令:"
        echo "pip install --index-url https://test.pypi.org/simple/ mcp-server-aliyun-observability==$version"
    else
        log_info "安装命令:"
        echo "pip install mcp-server-aliyun-observability==$version"
    fi
}

# 主要部署流程
deploy() {
    local current_version=$(get_version)
    log_info "开始部署流程 - 版本 $current_version"
    
    # 检查依赖
    check_dependencies
    
    # 运行检查
    if [ "$SKIP_CHECKS" != "true" ]; then
        check_package
    fi
    
    # 运行测试
    if [ "$SKIP_TESTS" != "true" ]; then
        run_tests
    fi
    
    # 清理并构建
    clean_build
    build_package
    
    # 上传
    upload_package
    
    log_success "部署完成！版本 $current_version 已发布"
}

# 解析命令行参数
USE_TEST_PYPI=false
DRY_RUN=false
SKIP_TESTS=false
SKIP_CHECKS=false
COMMAND="deploy"

while [[ $# -gt 0 ]]; do
    case $1 in
        --test-pypi)
            USE_TEST_PYPI=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        --skip-checks)
            SKIP_CHECKS=true
            shift
            ;;
        --version)
            get_version
            exit 0
            ;;
        build|upload|deploy|clean|check|test|help)
            COMMAND=$1
            shift
            ;;
        *)
            log_error "未知参数: $1"
            show_help
            exit 1
            ;;
    esac
done

# 切换到项目根目录
cd "$PROJECT_ROOT"

# 执行命令
case $COMMAND in
    build)
        check_dependencies
        clean_build
        build_package
        ;;
    upload)
        check_dependencies
        upload_package
        ;;
    deploy)
        deploy
        ;;
    clean)
        clean_build
        ;;
    check)
        check_dependencies
        check_package
        ;;
    test)
        check_dependencies
        run_tests
        ;;
    help)
        show_help
        ;;
    *)
        log_error "未知命令: $COMMAND"
        show_help
        exit 1
        ;;
esac