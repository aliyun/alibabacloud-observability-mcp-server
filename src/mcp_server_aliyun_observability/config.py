"""通用配置类"""

import os


class Config:
    """MCP服务器的通用配置类"""

    # 重试配置
    MAX_RETRY_ATTEMPTS = int(os.getenv("MAX_RETRY_ATTEMPTS", "1"))  # 默认重试1次
    RETRY_WAIT_SECONDS = int(os.getenv("RETRY_WAIT_SECONDS", "1"))  # 重试等待时间（秒）

    # 超时配置
    READ_TIMEOUT_MS = int(
        os.getenv("READ_TIMEOUT_MS", "610000")
    )  # 读取超时（毫秒），默认10秒
    CONNECT_TIMEOUT_MS = int(
        os.getenv("CONNECT_TIMEOUT_MS", "30000")
    )  # 连接超时（毫秒），默认10秒

    # 调试配置
    DEBUG_MODE = os.getenv("DEBUG_MODE", "false").lower() in ["true", "1", "yes", "on"]

    @classmethod
    def is_test_mode(cls) -> bool:
        """检查是否在测试模式下运行"""
        # 通过环境变量或pytest标记来判断
        return os.getenv("PYTEST_CURRENT_TEST") is not None or os.getenv(
            "TEST_MODE", "false"
        ).lower() in ["true", "1", "yes", "on"]

    @classmethod
    def get_retry_attempts(cls) -> int:
        """获取重试次数，测试模式下返回1"""
        if cls.is_test_mode():
            return 1
        return cls.MAX_RETRY_ATTEMPTS

    @classmethod
    def get_timeouts(cls) -> tuple[int, int]:
        """获取超时配置，返回(读取超时, 连接超时)"""
        return cls.READ_TIMEOUT_MS, cls.CONNECT_TIMEOUT_MS
