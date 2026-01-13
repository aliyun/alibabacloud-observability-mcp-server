"""IaaS Toolkit 测试环境配置

提供真实 SLS 测试环境的 setup 和 teardown 闭环：
- 自动创建测试用的 Project 和 Logstore
- 写入测试日志数据
- 测试完成后自动清理资源
"""
import os
import time
import uuid
from typing import Generator

import dotenv
import pytest
from alibabacloud_sls20201230.client import Client as SLSClient
from alibabacloud_sls20201230.models import (
    CreateLogStoreRequest,
    CreateProjectRequest,
    CreateIndexRequest,
    DeleteProjectRequest,
    Index,
    IndexLine,
    IndexKey,
    PutLogsRequest,
    PutLogsHeaders,
    LogGroup,
    LogItem,
    LogContent,
)
from alibabacloud_tea_openapi import models as open_api_models
from alibabacloud_tea_util import models as util_models
from mcp.server.fastmcp import Context, FastMCP
from mcp.shared.context import RequestContext

from mcp_server_aliyun_observability.server import create_lifespan
from mcp_server_aliyun_observability.toolkits.iaas.toolkit import register_iaas_tools
from mcp_server_aliyun_observability.utils import SLSClientWrapper, CredentialWrapper
from mcp_server_aliyun_observability.logger import get_logger

# 加载环境变量
dotenv.load_dotenv()

logger = get_logger()


class SLSTestEnvironment:
    """SLS 测试环境管理器
    
    负责创建和销毁测试用的 SLS 资源，包括：
    - Project
    - Logstore
    - 索引配置
    - 测试日志数据
    """
    
    def __init__(
        self,
        region_id: str = "cn-hangzhou",
        project_prefix: str = "mcp-test",
        logstore_name: str = "test-logs",
    ):
        self.region_id = region_id
        self.project_prefix = project_prefix
        self.logstore_name = logstore_name
        
        # 生成唯一的项目名称（避免冲突）
        unique_id = uuid.uuid4().hex[:8]
        self.project_name = f"{project_prefix}-{unique_id}"
        
        # 从环境变量获取凭证
        self.access_key_id = os.getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
        self.access_key_secret = os.getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
        
        if not self.access_key_id or not self.access_key_secret:
            raise ValueError(
                "请设置环境变量 ALIBABA_CLOUD_ACCESS_KEY_ID 和 ALIBABA_CLOUD_ACCESS_KEY_SECRET"
            )
        
        # 打印调试信息（隐藏敏感信息）
        masked_key_id = self.access_key_id[:4] + "****" + self.access_key_id[-4:] if len(self.access_key_id) > 8 else "****"
        logger.info(f"使用 Access Key ID: {masked_key_id}, Region: {region_id}")
        
        # 创建 SLS 客户端
        self.client = self._create_client()
        
        # 资源创建状态
        self._project_created = False
        self._logstore_created = False
        self._index_created = False
        
    def _create_client(self) -> SLSClient:
        """创建 SLS 客户端"""
        config = open_api_models.Config(
            access_key_id=self.access_key_id,
            access_key_secret=self.access_key_secret,
            endpoint=f"{self.region_id}.log.aliyuncs.com",
        )
        return SLSClient(config)
    
    def _get_runtime_options(self) -> util_models.RuntimeOptions:
        """获取运行时选项"""
        runtime = util_models.RuntimeOptions()
        runtime.read_timeout = 60000
        runtime.connect_timeout = 60000
        return runtime
    
    def setup(self) -> dict:
        """设置测试环境
        
        Returns:
            包含测试环境信息的字典
        """
        logger.info(f"开始设置 SLS 测试环境: project={self.project_name}")
        
        try:
            # 1. 创建 Project
            self._create_project()
            
            # 等待 Project 创建完成
            time.sleep(3)
            
            # 2. 创建 Logstore
            self._create_logstore()
            
            # 等待 Logstore 创建完成
            time.sleep(3)
            
            # 3. 创建索引
            self._create_index()
            
            # 等待索引创建完成（SLS 索引创建需要较长时间才能生效）
            logger.info("等待索引生效（约 30 秒）...")
            time.sleep(30)
            
            # 4. 写入测试日志
            self._put_test_logs()
            
            # 等待日志写入并可查询（日志服务有延迟）
            logger.info("等待日志数据可查询（约 5 秒）...")
            time.sleep(5)
            
            logger.info(f"SLS 测试环境设置完成: project={self.project_name}, logstore={self.logstore_name}")
            
            return {
                "project": self.project_name,
                "logstore": self.logstore_name,
                "region_id": self.region_id,
            }
            
        except Exception as e:
            logger.error(f"设置测试环境失败: {e}")
            # 尝试清理已创建的资源
            self.teardown()
            raise
    
    def _create_project(self):
        """创建 SLS Project"""
        logger.info(f"创建 Project: {self.project_name}")
        
        request = CreateProjectRequest(
            project_name=self.project_name,
            description="MCP Server Integration Test Project - Auto Created",
        )
        
        self.client.create_project_with_options(
            request, headers={}, runtime=self._get_runtime_options()
        )
        self._project_created = True
        logger.info(f"Project 创建成功: {self.project_name}")
    
    def _create_logstore(self):
        """创建 Logstore"""
        logger.info(f"创建 Logstore: {self.logstore_name}")
        
        request = CreateLogStoreRequest(
            logstore_name=self.logstore_name,
            ttl=1,  # 保留 1 天
            shard_count=2,  # 2 个 shard
            auto_split=True,
            max_split_shard=64,
        )
        
        self.client.create_log_store_with_options(
            self.project_name, request, headers={}, runtime=self._get_runtime_options()
        )
        self._logstore_created = True
        logger.info(f"Logstore 创建成功: {self.logstore_name}")
    
    def _create_index(self):
        """创建索引"""
        logger.info(f"创建索引: {self.logstore_name}")
        
        # 通用分词符
        default_token = [",", " ", "'", "\"", ";", "=", "(", ")", "[", "]", "{", "}", "?", "@", "&", "<", ">", "/", ":", "\n", "\t", "\r"]
        
        # 创建全文索引配置
        line_config = IndexLine(
            case_sensitive=False,
            chn=True,  # 支持中文分词
            token=default_token,
        )
        
        # 创建字段索引配置（每个 IndexKey 都需要 token）
        keys_config = {
            "level": IndexKey(type="text", alias="", doc_value=True, case_sensitive=False, token=default_token),
            "message": IndexKey(type="text", alias="", doc_value=True, case_sensitive=False, token=default_token),
            "service": IndexKey(type="text", alias="", doc_value=True, case_sensitive=False, token=default_token),
            "trace_id": IndexKey(type="text", alias="", doc_value=True, case_sensitive=False, token=default_token),
            "request_id": IndexKey(type="text", alias="", doc_value=True, case_sensitive=False, token=default_token),
        }
        
        # 创建 Index 对象
        index_body = Index(
            line=line_config,
            keys=keys_config,
        )
        
        request = CreateIndexRequest(body=index_body)
        
        self.client.create_index_with_options(
            self.project_name, self.logstore_name, request, headers={}, runtime=self._get_runtime_options()
        )
        self._index_created = True
        logger.info(f"索引创建成功: {self.logstore_name}")
    
    def _put_test_logs(self, count: int = 250):
        """写入测试日志数据
        
        Args:
            count: 写入的日志条数（默认 250 条，用于测试分页）
        """
        logger.info(f"写入测试日志: {count} 条")
        
        current_time = int(time.time())
        
        levels = ["INFO", "WARN", "ERROR", "DEBUG"]
        services = ["user-service", "order-service", "payment-service", "gateway"]
        
        # 分批写入（每批最多 4096 条）
        batch_size = 100
        for batch_start in range(0, count, batch_size):
            batch_end = min(batch_start + batch_size, count)
            log_items = []
            
            for i in range(batch_start, batch_end):
                level = levels[i % len(levels)]
                service = services[i % len(services)]
                
                # 创建日志内容
                contents = [
                    LogContent(key="level", value=level),
                    LogContent(key="message", value=f"Test log message {i}: This is a test log entry for pagination testing."),
                    LogContent(key="service", value=service),
                    LogContent(key="trace_id", value=f"trace-{uuid.uuid4().hex[:16]}"),
                    LogContent(key="request_id", value=f"req-{i:06d}"),
                ]
                
                # 创建日志条目
                log_item = LogItem(
                    contents=contents,
                    time=current_time - (count - i),  # 按时间递增
                )
                log_items.append(log_item)
            
            # 创建日志组
            log_group = LogGroup(
                log_items=log_items,
                source="mcp-integration-test",
                topic="test",
            )
            
            # 创建请求
            request = PutLogsRequest(body=log_group)
            
            # 使用简单的 put_logs 方法（SDK 会自动处理压缩）
            self.client.put_logs(
                self.project_name,
                self.logstore_name,
                request,
            )
            logger.info(f"写入日志批次 {batch_start // batch_size + 1}: {batch_end - batch_start} 条")
        
        logger.info(f"测试日志写入完成: 共 {count} 条")
    
    def teardown(self):
        """销毁测试环境"""
        logger.info(f"开始清理 SLS 测试环境: project={self.project_name}")
        
        errors = []
        
        # 1. 删除 Logstore（必须先删除 Logstore 才能删除 Project）
        if self._logstore_created:
            try:
                logger.info(f"删除 Logstore: {self.logstore_name}")
                self.client.delete_log_store_with_options(
                    self.project_name, self.logstore_name, headers={}, runtime=self._get_runtime_options()
                )
                logger.info(f"Logstore 删除成功: {self.logstore_name}")
            except Exception as e:
                logger.warning(f"删除 Logstore 失败: {e}")
                errors.append(f"Logstore: {e}")
        
        # 等待 Logstore 删除完成
        time.sleep(2)
        
        # 2. 删除 Project
        if self._project_created:
            try:
                logger.info(f"删除 Project: {self.project_name}")
                delete_request = DeleteProjectRequest()
                self.client.delete_project_with_options(
                    self.project_name, delete_request, headers={}, runtime=self._get_runtime_options()
                )
                logger.info(f"Project 删除成功: {self.project_name}")
            except Exception as e:
                logger.warning(f"删除 Project 失败: {e}")
                errors.append(f"Project: {e}")
        
        if errors:
            logger.warning(f"清理测试环境时遇到错误: {errors}")
        else:
            logger.info(f"SLS 测试环境清理完成: project={self.project_name}")


@pytest.fixture(scope="module")
def sls_test_env() -> Generator[dict, None, None]:
    """SLS 测试环境 fixture（模块级别）
    
    自动创建和销毁 SLS 测试资源。
    
    Yields:
        包含测试环境信息的字典：
        - project: 项目名称
        - logstore: 日志库名称
        - region_id: 区域 ID
    """
    # 检查是否启用集成测试
    if not os.getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"):
        pytest.skip("未设置 ALIBABA_CLOUD_ACCESS_KEY_ID，跳过集成测试")
    
    env = SLSTestEnvironment(
        region_id=os.getenv("SLS_TEST_REGION", "cn-hangzhou"),
        project_prefix="mcp-test",
        logstore_name="test-logs",
    )
    
    try:
        env_info = env.setup()
        yield env_info
    finally:
        env.teardown()


@pytest.fixture(scope="module")
def real_request_context(sls_test_env: dict) -> Context:
    """创建带有真实 SLS 客户端的请求上下文"""
    return Context(
        request_context=RequestContext(
            request_id="integration_test_request",
            meta=None,
            session=None,
            lifespan_context={
                "cms_client": None,
                "sls_client": SLSClientWrapper(
                    credential=CredentialWrapper(
                        access_key_id=os.getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"),
                        access_key_secret=os.getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET"),
                        knowledge_config=None,
                    ),
                ),
                "arms_client": None,
            },
        )
    )


@pytest.fixture(scope="module")
def real_mcp_server() -> FastMCP:
    """创建带有 IaaS 工具的 FastMCP 服务器"""
    server = FastMCP(
        "integration-test-server",
        lifespan=create_lifespan(
            credential=CredentialWrapper(
                access_key_id=os.getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"),
                access_key_secret=os.getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET"),
                knowledge_config=None,
            ),
        ),
    )
    register_iaas_tools(server)
    return server
