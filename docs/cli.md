目前我的服务启动是用 python -m mcp_server_aliyun_observability --transport streamable-http --transport-port 8080 --toolkits iaas
recent directory
我想支持一下cli,当用户pip install包之后，除了可以用上述命令，也支持一些操作,工具名就叫 cli命令叫aom,包括一些命令：
aom start --transport streamable-http --transport-port 8080 --toolkits iaas
aop list_tools --scope iaas


以上只是我的初步设想，你可以根据实际情况做调整，包括命令的命名，可以更加友好，类似于claude code那样