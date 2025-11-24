# 多阶段构建：构建阶段
FROM arms-deploy-registry.cn-shanghai.cr.aliyuncs.com/arms-deploy-repo/python:3.12-slim-bookworm AS builder

WORKDIR /app

# 优化：使用阿里云镜像源加速包管理器
RUN sed -i 's@deb.debian.org@mirrors.aliyun.com@g' /etc/apt/sources.list.d/debian.sources && \
    sed -i 's@security.debian.org@mirrors.aliyun.com@g' /etc/apt/sources.list.d/debian.sources

# 优化：配置pip使用阿里云镜像源
RUN pip config set global.index-url https://mirrors.aliyun.com/pypi/simple/ && \
    pip config set install.trusted-host mirrors.aliyun.com

# 一次性安装所有系统依赖，减少层数
RUN apt-get update && apt-get install -y --no-install-recommends \
    # 编译工具和基础依赖
    gcc g++ libc-dev libffi-dev \
    # 数学库依赖
    libgmp-dev libmpfr-dev libmpc-dev \
    # 网络和安全
    curl ca-certificates \
    && apt-get autoremove -y \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# 优化：先复制依赖文件，利用Docker层缓存
COPY requirements.txt pyproject.toml ./
COPY libs/ /app/libs/

# 优化：使用pip缓存和并行安装
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --no-deps -e ./libs/cms-20240330/ && \
    pip install --no-deps -e ./libs/sts-20150401/ && \
    pip install --no-cache-dir -r requirements.txt

# 复制源码（放在最后，避免代码变更影响依赖缓存）
COPY src/ ./src/
COPY README.md ./

# 安装主项目（无依赖模式，因为依赖已安装）
RUN pip install --no-deps -e .

# 最终阶段：运行时镜像
FROM arms-deploy-registry.cn-shanghai.cr.aliyuncs.com/arms-deploy-repo/python:3.12-slim-bookworm AS runtime

WORKDIR /app

# 优化：使用阿里云镜像源加速包管理器
RUN sed -i 's@deb.debian.org@mirrors.aliyun.com@g' /etc/apt/sources.list.d/debian.sources && \
    sed -i 's@security.debian.org@mirrors.aliyun.com@g' /etc/apt/sources.list.d/debian.sources

# 安装运行时依赖（仅必需的库）
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# 配置pip使用阿里云镜像源
RUN pip config set global.index-url https://mirrors.aliyun.com/pypi/simple/ && \
    pip config set install.trusted-host mirrors.aliyun.com

# 从构建阶段复制已安装的Python环境
COPY --from=builder /usr/local/lib/python3.12/site-packages/ /usr/local/lib/python3.12/site-packages/
COPY --from=builder /usr/local/bin/ /usr/local/bin/
COPY --from=builder /app/ /app/

# 创建日志目录
RUN mkdir -p /root/mcp_server_aliyun_observability

# 设置环境变量
ENV PYTHONUNBUFFERED=1

ENV TRANSPORT=sse
ENV HOST=0.0.0.0
ENV PORT=8080
ENV SCOPE=all
EXPOSE $PORT

# 声明日志卷
VOLUME ["/root/mcp_server_aliyun_observability"]

# 运行服务器 - 使用shell形式以正确展开环境变量，指定端口8080
ENTRYPOINT python -m mcp_server_aliyun_observability --transport $TRANSPORT --transport-port $PORT --host $HOST --scope $SCOPE