MCP 新的重构需求

## 需求
1. 对MCP的toolkits做重新拆分，总共分为三个package,iaas,pass以及ai 这三层
### iaas层的工具包含两个:
1. text_to_sql
2. execute_sql
3. execute_promql
这两个其他可以去掉

### PAAS层的工具都是来自于 umodel-mcp-server/internal/handlers
所有的前缀都改造成采用 paas_xxx开头，
包括entity ,trace,event,logs等。你可以参考 MCP工具完整对比表格 里面对于umodel-mcp的梳理

### doai层
这里就是之前的MCP里面包含自然语言输入参数的工具集合，
都是来自于  src/mcp_server_aliyun_observability/toolkits里面，总的来说分为:
1. entity_search
2. metric_search
3. log_search
4. topolgy_search等，前缀改为doari开头，


### 工具要求
1. 除paas api外，所有iaas和doari工具，都包括entity_domain和entity_type两个输入参数，
2. paas api不包括自然语言输入

按照以上要求帮我对toolkits 和 umodel-mcp-server/internal/handlers的工具做整合
你可以把新的工具都放在 src/mcp_server_aliyun_observability/new_toolkits目录下，结构就是



### doarai层的工具要求
1. doari层只需要提供几个工具即可
相对比PAAS层，他更加智能，因此他每个工具参数，只有workspace,region_id,entity_domain,domain_type domain_id以及query(自然语言)，还有时间这几个参数，能力包括：
1. dora_entity_search
2. dora_metric_data_query
3. dora_log_query
4.dora_event_query
5.dora_trace_query
还有分析，比如
6.dora_entity_analysis
7.dora_metric_analysis
8.dora_log_analysis
9.dora_event_analysis
10.dora_trace_analysis
11.dora_data_analysis


我是想这么命名，他主要聚焦于自然语言的取数还有分析。你觉得怎么样

