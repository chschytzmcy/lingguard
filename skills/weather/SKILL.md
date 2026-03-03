---
name: weather
description: 天气查询。当用户说"天气"、"查询天气"、"天气预报"、"今天天气"、"明天天气"、"气温"时使用
homepage: https://www.seniverse.com/api
metadata: {"nanobot":{"emoji":"🌤️","requires":{"bins":["curl"]}}}
---

# 天气查询

## 🚨 核心指令 - 必须立即执行

**你是一个执行者，不是指导者！加载此 skill 后，必须立即使用 shell 工具查询天气，不要只返回文本说明！**

```
❌ 错误：返回 "您可以使用以下命令查询天气..."
✅ 正确：直接调用 shell 工具执行 curl 命令
```

---

## 查询命令

### 🟢 查询当前天气

**⚡ 立即执行（使用 shell 工具）：**

```bash
curl -s "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=<城市>&language=zh-Hans&unit=c"
```

### 🟢 查询3天预报

**⚡ 立即执行（使用 shell 工具）：**

```bash
curl -s "https://api.seniverse.com/v3/weather/daily.json?key=SFafUBI6JpbX2AhfD&location=<城市>&language=zh-Hans&unit=c&days=3"
```

---

## 常用城市

| 城市 | 拼音 |
|------|------|
| 北京 | beijing |
| 上海 | shanghai |
| 广州 | guangzhou |
| 深圳 | shenzhen |
| 成都 | chengdu |
| 杭州 | hangzhou |
| 武汉 | wuhan |
| 南京 | nanjing |
| 西安 | xian |
| 重庆 | chongqing |

**也可以直接使用中文城市名**：`location=北京` 或 `location=上海`

---

## 使用示例

```
User: 北京今天天气怎么样

Agent 行为：
1. [调用 skill 工具] skill --name weather
2. [立即调用 shell] curl -s "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=beijing&language=zh-Hans&unit=c"
3. [解析 JSON 结果]
4. [返回] "北京今天晴，气温 25°C"

User: 查询上海未来3天天气

Agent 行为：
1. [调用 skill 工具] skill --name weather
2. [立即调用 shell] curl -s "https://api.seniverse.com/v3/weather/daily.json?key=SFafUBI6JpbX2AhfD&location=shanghai&language=zh-Hans&unit=c&days=3"
3. [解析结果并格式化输出]
```

---

## 返回数据格式

天气实况返回：
```json
{
  "results": [{
    "location": {"name": "北京"},
    "now": {
      "text": "晴",
      "code": "0",
      "temperature": "25"
    }
  }]
}
```

---

## 输出格式

向用户报告天气时，使用友好的格式：

```
🌤️ <城市>天气

当前：<天气状况>，<温度>°C
湿度：<湿度>%
风向：<风向> <风级>

（如有预报）
明日：<天气状况>，<低温>°C ~ <高温>°C
```
