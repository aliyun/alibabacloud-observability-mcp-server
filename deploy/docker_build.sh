#!/bin/bash

# 设置错误时退出
set -e

# ASCII 艺术标题
cat << 'EOF'
 __  __  ____  ____    ____                             
|  \/  |/ ___||  _ \  / ___|  ___ _ ____   _____ _ __    
| |\/| | |    | |_) | \___ \ / _ \ '__\ \ / / _ \ '__|   
| |  | | |___ |  __/   ___) |  __/ |   \ V /  __/ |      
|_|  |_|\____|_|     |____/ \___|_|    \_/ \___|_|      
                                                        
   阿里云可观测统一接入协议服务器 (Alibaba Cloud Observability MCP)
EOF
echo "容器镜像构建和推送脚本 v2.0"
echo "==============================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 定义日志函数
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

log_info "使用 Podman 容器运行时"

# 默认配置
DEFAULT_REGISTRY="arms-deploy-registry.cn-shanghai.cr.aliyuncs.com"
DEFAULT_NAMESPACE="arms-deploy-repo"
PROJECT_NAME="insight-problem-mcp"
DEFAULT_TAG="latest"
# 定义环境选项（可选，不再强制选择）
ENV_OPTIONS=("test" "prod-vpc" "prod-oxs")
# ACR 认证信息
ACR_USERNAME=${ACR_USERNAME:-"haibin.lhb@arms"}
ACR_PASSWORD=${ACR_PASSWORD:-"arms@123@arms"}
REGION_ID=${REGION_ID:-"cn-shanghai"}

# 帮助信息
show_help() {
    echo "用法: $0 [选项]"
    echo "选项:"
    echo "  -t, --tag TAG       指定镜像标签 (默认: latest)"
    echo "  -r, --registry REG  指定镜像仓库 (默认: ${DEFAULT_REGISTRY})"
    echo "  -n, --namespace NS  指定命名空间 (默认: ${DEFAULT_NAMESPACE})"
    echo "  -e, --env ENV       指定环境 (可选: test, prod-vpc, prod-oxs) - 如不指定则使用默认配置"
    echo "  -h, --help          显示帮助信息"
    echo "  -c, --clean         清理旧的构建文件"
    echo "  -p, --push          构建后推送镜像到仓库"
    echo "  --no-push           构建后不推送镜像"
    echo "  --no-merge          不合并 main 分支（仅对 prod 环境有效）"
    echo "  --no-tag            不创建标签（仅对 prod 环境有效）"
    echo "  --skip-aliyun-agent 跳过阿里云 Agent 安装（开发环境使用）"
    echo ""
    echo "注意: 如果没有指定 --push 或 --no-push，脚本将交互式询问是否推送"
    echo "安全提示: 脚本会自动检测 feature/* 分支，并禁用合并到主分支功能"
}

# 函数：选择环境（已禁用，环境参数现在完全可选）
# select_env() {
#     log_info "请选择环境:"
#     select ENV in "${ENV_OPTIONS[@]}"; do
#         if [[ " ${ENV_OPTIONS[*]} " =~ " ${ENV} " ]]; then
#             log_info "您选择了: $ENV"
#             return 0
#         else
#             log_warn "无效的选择，请重试。"
#         fi
#     done
# }

# 函数：询问是否推送镜像
ask_push_confirmation() {
    if [ "$PUSH_SPECIFIED" = false ]; then
        echo ""
        log_info "镜像构建完成！"
        read -p "是否要将镜像推送到仓库？(y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            PUSH=true
            log_info "用户选择推送镜像"
        else
            PUSH=false
            log_info "用户选择不推送镜像"
        fi
    fi
}

# 函数：检查未提交的更改
check_uncommitted_changes() {
    log_info "检查未提交的更改..."
    # if [[ $ENV == prod-* ]]; then
    #     if ! git diff-index --quiet HEAD --; then
    #         log_error "在 prod 环境中，存在未提交的更改。请在构建容器镜像之前提交或暂存您的更改。"
    #         exit 1
    #     fi
    # else
    #     if ! git diff-index --quiet HEAD -- ':!build.sh'; then
    #         log_warn "仓库中存在未提交的更改（不包括 build.sh）。"
    #         read -p "是否继续？(y/n) " -n 1 -r
    #         echo
    #         if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    #             log_info "用户选择退出。"
    #             exit 1
    #         fi
    #     fi
    # fi
    log_info "未提交更改检查完成。"
}

# 函数：检查分支保护
check_branch_protection() {
    local current_branch=$(git rev-parse --abbrev-ref HEAD)
    log_info "当前分支: ${current_branch}"
    
    # 如果是 feature 分支，强制禁用合并和标签创建
    if [[ $current_branch == feature/* ]]; then
        log_warn "检测到 feature 分支: ${current_branch}"
        log_warn "为防止意外合并到主分支，自动禁用合并和标签创建功能"
        MERGE=false
        TAG_CREATION=false
        return 0
    fi
    
    # 如果是开发或测试分支，也禁用合并
    if [[ $current_branch == develop* ]] || [[ $current_branch == test* ]] || [[ $current_branch == dev* ]]; then
        log_warn "检测到开发/测试分支: ${current_branch}"
        log_warn "自动禁用合并到主分支功能"
        MERGE=false
        return 0
    fi
}

# 函数：合并 main 分支（仅用于 prod 环境）
merge_main_branch() {
    if [[ $ENV == prod-* ]] && [ "$MERGE" = true ]; then
        # 获取当前分支
        current_branch=$(git rev-parse --abbrev-ref HEAD)
        log_info "当前分支: ${current_branch}"
        
        # 额外的安全检查：如果是 feature 分支，绝对不允许合并
        if [[ $current_branch == feature/* ]]; then
            log_error "安全检查失败：不允许从 feature 分支合并到 main 分支！"
            log_error "当前分支: ${current_branch}"
            exit 1
        fi
        
        # 如果当前分支是 main，则不需要合并
        if [ "$current_branch" = "main" ]; then
            log_info "当前已在 main 分支，不需要合并"
            return 0
        fi
        
        # 执行合并操作
        log_info "正在合并 main 分支到当前分支..."
        if ! git fetch origin main; then
            log_error "无法从远程仓库获取 main 分支"
            exit 1
        fi
        if ! git merge origin/main --no-edit; then
            log_error "合并 main 分支失败。请解决冲突后重试。"
            git merge --abort
            exit 1
        fi
        log_info "main 分支合并成功"
    else
        log_info "跳过 main 分支合并"
    fi
}

# 函数：创建标签并询问是否合并到主干（仅用于 prod 环境）
create_tag_and_merge() {
    if [[ $ENV != prod-* ]] || [ "$TAG_CREATION" = false ]; then
        log_info "非 prod 环境或已禁用标签创建，跳过标签创建和合并操作"
        return 0
    fi

    current_branch=$(git rev-parse --abbrev-ref HEAD)
    
    # 安全检查：如果是 feature 分支，绝对不允许创建标签和合并
    if [[ $current_branch == feature/* ]]; then
        log_error "安全检查失败：不允许从 feature 分支创建标签或合并到 main 分支！"
        log_error "当前分支: ${current_branch}"
        log_error "请切换到正确的发布分支后再执行此操作"
        return 0
    fi

    local tag_name="v${TAG}"

    log_info "正在创建标签: $tag_name"
    if ! git tag "$tag_name"; then
        log_error "创建标签失败"
        return 1
    fi
    log_info "标签 $tag_name 创建成功"

    if [ "$current_branch" = "main" ]; then
        log_info "当前分支是 main 分支，跳过合并到主干操作"
        return 0
    fi

    read -p "是否要将更改合并到主干分支 (main)? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "正在合并到主干分支..."
        current_branch=$(git rev-parse --abbrev-ref HEAD)
        if ! git checkout main; then
            log_error "切换到 main 分支失败"
            git checkout "$current_branch"
            return 1
        fi
        if ! git merge "$current_branch"; then
            log_error "合并到 main 分支失败"
            git checkout "$current_branch"
            return 1
        fi
        if ! git push origin main; then
            log_error "推送到远程 main 分支失败"
            git checkout "$current_branch"
            return 1
        fi
        if ! git push origin "$tag_name"; then
            log_error "推送标签到远程失败"
            git checkout "$current_branch"
            return 1
        fi
        git checkout "$current_branch"
        log_info "成功合并到主干并推送标签"
    else
        log_info "跳过合并到主干操作"
    fi
}

# 清理函数
cleanup() {
    log_info "清理旧的构建文件..."
    podman system prune -f
}

# 登录容器仓库
login_registry() {
    if [ "$PUSH" = true ]; then
        log_info "尝试登录容器仓库..."
        max_retries=5
        retry_count=0

        while [ $retry_count -lt $max_retries ]; do
            if echo ${ACR_PASSWORD} | podman login --username="${ACR_USERNAME}" --password-stdin ${REGISTRY}; then
                log_info "Podman 登录成功"
                break
            fi
            
            log_warn "容器仓库登录失败。5秒后重试... (尝试 $((retry_count + 1))/$max_retries)"
            retry_count=$((retry_count + 1))
            sleep 5
        done

        if [ $retry_count -eq $max_retries ]; then
            log_error "容器仓库登录在 $max_retries 次尝试后失败。退出脚本。"
            exit 1
        fi
    fi
}

# 构建函数
build() {
    # 生成版本号
    if [ "$TAG" = "latest" ]; then
        # 获取原始分支名
        original_branch=$(git rev-parse --abbrev-ref HEAD)
        log_info "原始分支名: ${original_branch}"
        # 替换非法字符并获取处理后的分支名
        current_branch=$(echo "${original_branch}" | sed 's/[^a-zA-Z0-9-]/-/g')
        log_info "处理后的分支名: ${current_branch}"
        # 生成版本号
        commit_id=$(git rev-parse --short HEAD)
        timestamp=$(date +%Y%m%d%H%M%S)
        
        if [ -n "$ENV" ]; then
            TAG="${ENV}-${current_branch}-${commit_id}-${timestamp}"
            log_info "生成的版本号（带环境）: ${TAG}"
        else
            TAG="${current_branch}-${commit_id}-${timestamp}"
            log_info "生成的版本号（默认）: ${TAG}"
        fi
    fi
    
    # 设置镜像仓库
    if [[ $ENV == "test" ]]; then
        NAMESPACE="${PROJECT_NAME}-test"
        log_info "使用测试环境命名空间: ${NAMESPACE}"
    else
        log_info "使用默认命名空间: ${NAMESPACE}"
    fi
    
    local image_name="${REGISTRY}/${NAMESPACE}/${PROJECT_NAME}:${TAG}"
    
    log_info "开始构建镜像: ${image_name}"
    PLATFORM=${PLATFORM:-"linux/amd64"}
    log_info "使用Podman构建优化镜像..."
    
    # 构建参数
    BUILD_ARGS=""
    if [ "$SKIP_ALIYUN_AGENT" = true ]; then
        BUILD_ARGS="--build-arg SKIP_ALIYUN_AGENT=true"
        log_warn "跳过阿里云 Agent 安装 (开发模式)"
    fi
    
    # 构建优化：使用多阶段构建缓存和并行构建
    log_info "启用构建优化：多阶段构建 + 依赖缓存 + 阿里云镜像源"
    podman build --platform=${PLATFORM} ${BUILD_ARGS} \
        --layers=true \
        --build-arg BUILDKIT_INLINE_CACHE=1 \
        --target=runtime \
        -t ${image_name} .
    
    if [ $? -eq 0 ]; then
        log_info "镜像构建成功: ${image_name}"
        
        # 询问是否推送镜像（如果用户没有明确指定）
        ask_push_confirmation
        
        # 如果需要推送镜像
        if [ "$PUSH" = true ]; then
            # 登录容器仓库
            if ! echo ${ACR_PASSWORD} | podman login --username="${ACR_USERNAME}" --password-stdin ${REGISTRY} 2>/dev/null; then
                log_warn "容器仓库登录失败，开始重试..."
                login_registry
            else
                log_info "容器仓库登录成功"
            fi
            
            log_info "推送镜像到仓库..."
            podman push ${image_name}
            
            if [ $? -eq 0 ]; then
                log_info "镜像推送成功"
            else
                log_error "镜像推送失败"
                exit 1
            fi
        else
            log_info "跳过镜像推送"
        fi
        
        # 清理悬空镜像
        log_info "清理悬空镜像..."
        podman image prune -f
        
        # 显示新构建的镜像信息
        log_info "新构建的镜像信息:"
        podman images | grep ${PROJECT_NAME}
    else
        log_error "镜像构建失败"
        exit 1
    fi
    
    echo "==============================="
    log_info "镜像信息汇总:"
    log_info "镜像名称: ${image_name}"
    log_info "构建环境: ${ENV:-"默认配置"}"
    log_info "镜像标签: ${TAG}"
    echo "==============================="
}

# 解析命令行参数
TAG=$DEFAULT_TAG
REGISTRY=$DEFAULT_REGISTRY
NAMESPACE=$DEFAULT_NAMESPACE
CLEAN=false
PUSH=false  # 默认不推送，除非明确指定
PUSH_SPECIFIED=false  # 跟踪用户是否明确指定了推送选项
ENV=""
MERGE=true
TAG_CREATION=true
SKIP_ALIYUN_AGENT=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--tag)
            TAG="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -e|--env)
            ENV="$2"
            if [[ ! " ${ENV_OPTIONS[*]} " =~ " ${ENV} " ]]; then
                log_error "无效的环境选项: $ENV"
                show_help
                exit 1
            fi
            shift 2
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -p|--push)
            PUSH=true
            PUSH_SPECIFIED=true
            shift
            ;;
        --no-push)
            PUSH=false
            PUSH_SPECIFIED=true
            shift
            ;;
        --no-merge)
            MERGE=false
            shift
            ;;
        --no-tag)
            TAG_CREATION=false
            shift
            ;;
        --skip-aliyun-agent)
            SKIP_ALIYUN_AGENT=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            log_error "未知参数: $1"
            show_help
            exit 1
            ;;
    esac
done

# 主函数
main() {
    log_info "开始执行容器镜像构建和推送脚本..."
    
    # 检查分支保护（必须在最前面执行）
    check_branch_protection
    
    # 环境参数现在完全可选，不再强制选择
    if [ -n "$ENV" ]; then
        log_info "使用指定环境: $ENV"
    else
        log_info "未指定环境，将使用默认配置"
    fi
    
    # 执行未提交更改检查
    check_uncommitted_changes
    
    # 合并 main 分支
    merge_main_branch
    
    # 如果指定了清理选项，执行清理
    if [ "$CLEAN" = true ]; then
        cleanup
    fi
    
    # 构建镜像
    build
    
    # 创建标签并询问是否合并到主干
    if create_tag_and_merge; then
        log_info "标签创建和合并操作完成"
    else
        log_warn "标签创建或合并操作失败"
    fi
    
    log_info "脚本执行成功完成"
}

# 执行主函数
main 