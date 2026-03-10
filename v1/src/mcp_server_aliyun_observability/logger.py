import logging
import logging.handlers
import os
from datetime import datetime
from os import getenv
from pathlib import Path
from typing import Any, Literal, Optional, Union

from rich.console import Console
from rich.logging import RichHandler
from rich.text import Text

LOGGER_NAME = "mcp_server_aliyun_observability"

# Define custom styles for log sources
LOG_STYLES = {
    "server": {
        "debug": "green",
        "info": "blue",
    },
}


class ColoredRichHandler(RichHandler):
    def __init__(self, *args, source_type: Optional[str] = None, **kwargs):
        super().__init__(*args, **kwargs)
        self.source_type = source_type

    def get_level_text(self, record: logging.LogRecord) -> Text:
        # Return empty Text if message is empty
        if not record.msg:
            return Text("")

        level_name = record.levelname.lower()
        if self.source_type and self.source_type in LOG_STYLES:
            if level_name in LOG_STYLES[self.source_type]:
                color = LOG_STYLES[self.source_type][level_name]
                return Text(record.levelname, style=color)
        return super().get_level_text(record)


class MCPLogger(logging.Logger):
    def __init__(self, name: str, level: int = logging.NOTSET):
        super().__init__(name, level)

    def debug(self, msg: str, center: bool = False, symbol: str = "*", *args, **kwargs):
        if center:
            msg = center_header(str(msg), symbol)
        super().debug(msg, *args, **kwargs)

    def info(self, msg: str, center: bool = False, symbol: str = "*", *args, **kwargs):
        if center:
            msg = center_header(str(msg), symbol)
        super().info(msg, *args, **kwargs)


def setup_file_handler(logger_instance: logging.Logger) -> None:
    """设置文件处理器，将日志写入到指定文件"""
    # 创建日志目录
    log_dir = Path.home() / "mcp_server_aliyun_observability"
    log_dir.mkdir(exist_ok=True)

    # 生成日志文件名（包含日期）
    today = datetime.now().strftime("%Y%m%d")
    log_file = log_dir / f"mcp_server_{today}.log"

    # 创建文件处理器
    file_handler = logging.FileHandler(log_file, encoding='utf-8')
    file_handler.setLevel(logging.DEBUG)

    # 设置文件日志格式
    file_formatter = logging.Formatter(
        fmt='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        datefmt='%Y-%m-%d %H:%M:%S'
    )
    file_handler.setFormatter(file_formatter)

    # 添加到logger
    logger_instance.addHandler(file_handler)


def build_logger(logger_name: str, source_type: Optional[str] = None) -> Any:
    # Set the custom logger class as the default for this logger
    logging.setLoggerClass(MCPLogger)

    # Create logger with custom class
    _logger = logging.getLogger(logger_name)

    # Reset logger class to default to avoid affecting other loggers
    logging.setLoggerClass(logging.Logger)

    # 创建自定义控制台，设置合适的宽度
    try:
        import shutil

        terminal_width = shutil.get_terminal_size().columns
        # 确保最小宽度，避免过窄
        console_width = max(terminal_width, 120)
    except Exception:
        console_width = 120  # 默认宽度

    console = Console(
        width=console_width,
        force_terminal=True,
        no_color=False,
        legacy_windows=False,
    )

    # https://rich.readthedocs.io/en/latest/reference/logging.html#rich.logging.RichHandler
    # https://rich.readthedocs.io/en/latest/logging.html#handle-exceptions
    rich_handler = ColoredRichHandler(
        show_time=True,
        rich_tracebacks=True,
        show_path=True if getenv("MCP_DEBUG") == "true" else False,
        tracebacks_show_locals=False,
        source_type=source_type or "server",
        console=console,  # 使用自定义控制台
        omit_repeated_times=False,
        keywords=[],  # 关键字高亮列表
        markup=False,  # 禁用标记，避免格式冲突
    )
    rich_handler.setFormatter(
        logging.Formatter(
            fmt="%(message)s",
            datefmt="[%X]",
        )
    )

    _logger.addHandler(rich_handler)

    # 添加文件处理器
    setup_file_handler(_logger)

    _logger.setLevel(logging.INFO)
    _logger.propagate = False
    return _logger


# 创建统一的logger实例
logger: MCPLogger = build_logger(LOGGER_NAME, source_type="server")

debug_on: bool = False
debug_level: Literal[1, 2] = 1


def set_log_level_to_debug(source_type: Optional[str] = None, level: Literal[1, 2] = 1):
    """设置日志级别为DEBUG"""
    _logger = logging.getLogger(LOGGER_NAME if source_type is None else f"{LOGGER_NAME}-{source_type}")
    _logger.setLevel(logging.DEBUG)

    global debug_on
    debug_on = True

    global debug_level
    debug_level = level


def set_log_level_to_info(source_type: Optional[str] = None):
    """设置日志级别为INFO"""
    _logger = logging.getLogger(LOGGER_NAME if source_type is None else f"{LOGGER_NAME}-{source_type}")
    _logger.setLevel(logging.INFO)

    global debug_on
    debug_on = False


def center_header(message: str, symbol: str = "*") -> str:
    """将消息居中显示"""
    try:
        import shutil

        terminal_width = shutil.get_terminal_size().columns
    except Exception:
        terminal_width = 80  # fallback width

    header = f" {message} "
    return f"{header.center(terminal_width - 20, symbol)}"


def log_debug(msg, center: bool = False, symbol: str = "*", log_level: Literal[1, 2] = 1, *args, **kwargs):
    """记录DEBUG级别日志"""
    global logger
    global debug_on
    global debug_level

    if debug_on:
        if debug_level >= log_level:
            logger.debug(msg, center, symbol, *args, **kwargs)


def log_info(msg, center: bool = False, symbol: str = "*", *args, **kwargs):
    """记录INFO级别日志"""
    global logger
    logger.info(msg, center, symbol, *args, **kwargs)


def log_warning(msg, *args, **kwargs):
    """记录WARNING级别日志"""
    global logger
    logger.warning(msg, *args, **kwargs)


def log_error(msg, *args, **kwargs):
    """记录ERROR级别日志"""
    global logger
    logger.error(msg, *args, **kwargs)


def log_exception(msg, *args, **kwargs):
    """记录异常日志"""
    global logger
    logger.exception(msg, *args, **kwargs)


# 便捷函数：获取logger实例
def get_logger() -> MCPLogger:
    """获取日志器实例"""
    return logger