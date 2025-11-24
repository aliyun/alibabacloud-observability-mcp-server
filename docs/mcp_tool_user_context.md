


def call_data_query(
    ctx: Context,
    query: str,
    region_id: str,
    workspace: str,
    user_context: Optional[List[Dict[str, Any]]] = None,
    error_message_prefix: str = "查询失败",
    client_type: str = "sls_client",
) -> Dict[str, Any]:


对v2/下面的MCP工具， 需要添加user_context参数， 这个参数是用来传递给AI工具的， 用来描述用户想要查询的实体，如果实现是用call_data_query实现的

这里的user_context应该是个数组， 这里的type, entity从entity selector里面提取
如下面的格式:
```json
[
   {
    "type":"entity",
    "entity_id":"xxxxx",
    "entity_type":"xxxxx",
    "domain":"xxxxx",
   }

]

```