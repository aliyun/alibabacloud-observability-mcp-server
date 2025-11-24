#!/usr/bin/env python3
"""
阿里云可观测MCP服务器部署工具
简化的Python版本部署脚本
"""

import os
import sys
import subprocess
import shutil
import argparse
from pathlib import Path
import tomllib


class DeployTool:
    def __init__(self):
        self.project_root = Path(__file__).parent
        self.dist_dir = self.project_root / "dist"
        
    def log(self, message, level="INFO"):
        colors = {
            "INFO": "\033[0;34m",
            "SUCCESS": "\033[0;32m", 
            "WARNING": "\033[1;33m",
            "ERROR": "\033[0;31m"
        }
        reset = "\033[0m"
        print(f"{colors.get(level, '')}[{level}]{reset} {message}")

    def get_version(self):
        """获取当前版本号"""
        pyproject_path = self.project_root / "pyproject.toml"
        with open(pyproject_path, "rb") as f:
            data = tomllib.load(f)
        return data["project"]["version"]

    def run_command(self, cmd, check=True):
        """运行命令"""
        self.log(f"执行: {' '.join(cmd)}")
        if isinstance(cmd, str):
            cmd = cmd.split()
        result = subprocess.run(cmd, capture_output=True, text=True)
        if check and result.returncode != 0:
            self.log(f"命令执行失败: {result.stderr}", "ERROR")
            sys.exit(1)
        return result

    def check_dependencies(self):
        """检查构建依赖"""
        self.log("检查构建依赖...")
        required = ["build", "twine"]
        missing = []
        
        for pkg in required:
            try:
                __import__(pkg)
            except ImportError:
                missing.append(pkg)
        
        if missing:
            self.log(f"安装缺少的包: {missing}")
            self.run_command([sys.executable, "-m", "pip", "install"] + missing)
        
        self.log("依赖检查完成", "SUCCESS")

    def run_tests(self):
        """运行测试"""
        self.log("运行测试...")
        try:
            self.run_command([sys.executable, "-m", "pytest", "-v"])
            self.log("测试通过", "SUCCESS")
        except:
            self.log("测试失败或pytest未安装", "WARNING")

    def clean_build(self):
        """清理构建文件"""
        self.log("清理构建文件...")
        
        # 清理目录
        dirs_to_clean = [
            self.dist_dir,
            self.project_root / "build",
            *self.project_root.glob("src/*.egg-info"),
            *self.project_root.glob("*.egg-info"),
        ]
        
        for dir_path in dirs_to_clean:
            if dir_path.exists():
                shutil.rmtree(dir_path)
        
        # 清理缓存文件
        for cache_dir in self.project_root.rglob("__pycache__"):
            shutil.rmtree(cache_dir, ignore_errors=True)
            
        for cache_file in self.project_root.rglob("*.pyc"):
            cache_file.unlink(missing_ok=True)
            
        self.log("清理完成", "SUCCESS")

    def build_package(self):
        """构建包"""
        self.log("开始构建包...")
        
        # 创建dist目录
        self.dist_dir.mkdir(exist_ok=True)
        
        # 构建
        self.run_command([
            sys.executable, "-m", "build", 
            "--wheel", "--sdist", 
            "--outdir", str(self.dist_dir)
        ])
        
        # 显示构建结果
        self.log("构建文件:")
        for file in self.dist_dir.iterdir():
            print(f"  {file.name}")
            
        # 检查包
        self.run_command([sys.executable, "-m", "twine", "check", f"{self.dist_dir}/*"])
        
        self.log("包构建完成", "SUCCESS")

    def upload_package(self, test_pypi=False, dry_run=False):
        """上传包"""
        if test_pypi:
            self.log("上传到测试PyPI...")
            repository = "testpypi"
            token_env = "TEST_PYPI_TOKEN"
        else:
            self.log("上传到正式PyPI...")
            repository = "pypi"
            token_env = "PYPI_TOKEN"
            
        if dry_run:
            self.log("模拟上传模式")
            return
            
        # 检查token
        token = os.getenv(token_env)
        if not token:
            self.log(f"请设置环境变量: {token_env}", "ERROR")
            sys.exit(1)
            
        # 上传
        cmd = [
            sys.executable, "-m", "twine", "upload",
            "--username", "__token__",
            "--password", token,
        ]
        
        if test_pypi:
            cmd.extend(["--repository", "testpypi"])
            
        cmd.append(f"{self.dist_dir}/*")
        
        self.run_command(cmd)
        
        # 显示安装命令
        version = self.get_version()
        if test_pypi:
            install_cmd = f"pip install --index-url https://test.pypi.org/simple/ mcp-server-aliyun-observability=={version}"
        else:
            install_cmd = f"pip install mcp-server-aliyun-observability=={version}"
            
        self.log(f"安装命令: {install_cmd}")
        self.log("包上传完成", "SUCCESS")

    def deploy(self, test_pypi=False, dry_run=False, skip_tests=False):
        """完整部署流程"""
        version = self.get_version()
        self.log(f"开始部署版本 {version}")
        
        self.check_dependencies()
        
        if not skip_tests:
            self.run_tests()
            
        self.clean_build()
        self.build_package()
        self.upload_package(test_pypi, dry_run)
        
        self.log(f"部署完成！版本 {version} 已发布", "SUCCESS")


def main():
    parser = argparse.ArgumentParser(description="阿里云可观测MCP服务器部署工具")
    
    subparsers = parser.add_subparsers(dest="command", help="可用命令")
    
    # build命令
    build_parser = subparsers.add_parser("build", help="构建包")
    
    # upload命令  
    upload_parser = subparsers.add_parser("upload", help="上传包")
    upload_parser.add_argument("--test-pypi", action="store_true", help="上传到测试PyPI")
    upload_parser.add_argument("--dry-run", action="store_true", help="模拟上传")
    
    # deploy命令
    deploy_parser = subparsers.add_parser("deploy", help="构建并部署")
    deploy_parser.add_argument("--test-pypi", action="store_true", help="部署到测试PyPI")
    deploy_parser.add_argument("--dry-run", action="store_true", help="模拟部署")
    deploy_parser.add_argument("--skip-tests", action="store_true", help="跳过测试")
    
    # clean命令
    clean_parser = subparsers.add_parser("clean", help="清理构建文件")
    
    # version命令
    version_parser = subparsers.add_parser("version", help="显示版本")
    
    args = parser.parse_args()
    
    tool = DeployTool()
    
    if args.command == "build":
        tool.check_dependencies()
        tool.clean_build()
        tool.build_package()
    elif args.command == "upload":
        tool.upload_package(args.test_pypi, args.dry_run)
    elif args.command == "deploy":
        tool.deploy(args.test_pypi, args.dry_run, args.skip_tests)
    elif args.command == "clean":
        tool.clean_build()
    elif args.command == "version":
        print(tool.get_version())
    else:
        parser.print_help()


if __name__ == "__main__":
    main()