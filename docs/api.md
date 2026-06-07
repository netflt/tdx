# TDX 股票数据 API 接口文档

## 基础信息

- **Base URL**: `http://localhost:8080`
- **Content-Type**: `application/json; charset=utf-8`
- **Web 测试页面**: `http://localhost:8080/`
- **接口文档**: `http://localhost:8080/api/docs`

---

## 响应格式

所有接口统一返回：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

- `code`: 0=成功, -1=失败
- `message`: 提示信息
- `data`: 数据内容

---

## 快速开始

```bash
# 一键部署
./deploy.sh

# 或指定端口
TDX_PORT=9090 ./deploy.sh

# Docker 部署
./deploy.sh docker

# 查看状态
./deploy.sh status

# 停止
./deploy.sh stop
```

---

## 接口总览

| 分类 | 接口数 | 接口 |
|------|--------|------|
| 行情 | 2 | quote, batch-quote |
| K线 | 5 | kline, kline-all, kline-all/tdx, kline-all/ths, kline-history |
| 指数 | 2 | index, index/all |
| 分时 | 1 | minute |
| 分时成交 | 4 | trade, trade-history, minute-trade-all, trade-history/full |
| 代码表 | 6 | codes, stock-codes, etf-codes, etf, search, market-count |
| 综合 | 1 | stock-info |
| 除权除息 | 1 | gbbq |
| 财务 | 1 | finance |
| 板块 | 2 | blocks, block-members |
| 集合竞价 | 1 | call-auction |
| 工作日 | 2 | workday, workday/range |
| 收益 | 1 | income |
| 任务 | 5 | tasks/pull-kline, tasks/pull-trade, tasks, tasks/{id}, tasks/{id}/cancel |
| 系统 | 2 | server-status, docs |

---

## 1. 行情

### 1.1 获取五档行情

`GET /api/quote`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码，逗号分隔多只 |

```
GET /api/quote?code=000001
GET /api/quote?code=000001,600519
```

响应示例：
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "Exchange": 0,
      "Code": "000001",
      "K": {
        "Last": 12250,    // 昨收价(厘)
        "Open": 12300,    // 开盘价(厘)
        "High": 12600,    // 最高价(厘)
        "Low": 12280,     // 最低价(厘)
        "Close": 12500    // 最新价(厘)
      },
      "TotalHand": 1235000,  // 总手
      "Amount": 156000000,   // 成交额(厘)
      "InsideDish": 520000,  // 内盘
      "OuterDisc": 715000,   // 外盘
      "BuyLevel": [],        // 买五档
      "SellLevel": []        // 卖五档
    }
  ]
}
```

### 1.2 批量获取行情

`POST /api/batch-quote`

请求体：
```json
{"codes": ["000001", "600519", "601318"]}
```

响应同 `/api/quote`。

---

## 2. K线

### 2.1 获取K线数据

`GET /api/kline`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| type | string | 否 | K线类型，默认 day |
| count | int | 否 | 返回条数，默认 100 |

K线类型(type)：
- `minute1` - 1分钟K线
- `minute5` - 5分钟K线
- `minute15` - 15分钟K线
- `minute30` - 30分钟K线
- `hour` - 60分钟/小时K线
- `day` - 日K线（默认）
- `week` - 周K线
- `month` - 月K线
- `quarter` - 季K线
- `year` - 年K线

```
GET /api/kline?code=000001&type=day&count=100
```

响应示例：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "Count": 100,
    "List": [
      {
        "Last": 12250,      // 昨收价(厘)
        "Open": 12300,      // 开盘价(厘)
        "High": 12600,      // 最高价(厘)
        "Low": 12280,       // 最低价(厘)
        "Close": 12500,     // 收盘价(厘)
        "Volume": 1235000,  // 成交量(手)
        "Amount": 156000000,// 成交额(厘)
        "Time": "2024-11-03T00:00:00Z",
        "UpCount": 0,       // 上涨数(指数有效)
        "DownCount": 0      // 下跌数(指数有效)
      }
    ]
  }
}
```

### 2.2 获取全量K线

`GET /api/kline-all`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| type | string | 否 | K线类型，默认 day |
| limit | int | 否 | 截取最近N条 |

返回全量K线数据，数据按时间正序排列。全量数据较大，建议配合 `limit` 控制响应大小。

### 2.3 通达信原始全量K线

`GET /api/kline-all/tdx`

参数同 `/api/kline-all`。返回通达信原始（不复权）K线，内部按800条一批拼接。响应包含 `meta` 字段标注数据源。

### 2.4 同花顺前复权全量K线

`GET /api/kline-all/ths`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| type | string | 否 | 仅支持 day/week/month，默认 day |
| limit | int | 否 | 截取最近N条 |

返回同花顺前复权K线。周/月K线由日K线合并转换。

### 2.5 获取历史K线

`GET /api/kline-history`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| type | string | 否 | K线类型，默认 day |
| start_date | string | 否 | 开始日期(YYYYMMDD) |
| end_date | string | 否 | 结束日期(YYYYMMDD) |
| limit | int | 否 | 返回条数，默认100，最大800 |

---

## 3. 指数

### 3.1 获取指数K线

`GET /api/index`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 指数代码(如 sh000001) |
| type | string | 否 | K线类型，默认 day |
| count | int | 否 | 返回条数，默认 100 |

常用指数代码：
- `sh000001` - 上证指数
- `sz399001` - 深证成指
- `sz399006` - 创业板指
- `sh000300` - 沪深300

### 3.2 获取指数全量K线

`GET /api/index/all`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 指数代码 |
| type | string | 否 | K线类型，默认 day |
| limit | int | 否 | 截取最近N条 |

---

## 4. 分时数据

### 4.1 获取分时数据

`GET /api/minute`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| date | string | 否 | 日期(YYYYMMDD)，默认当天 |

```
GET /api/minute?code=000001
GET /api/minute?code=000001&date=20260605
```

响应示例：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "Count": 240,
    "List": [
      {
        "Time": "09:31",
        "Price": 12300,
        "Number": 1500
      }
    ]
  }
}
```

---

## 5. 分时成交

### 5.1 获取分时成交

`GET /api/trade`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| date | string | 否 | 日期(YYYYMMDD)，默认当天 |

返回当日全量分时成交(自动分页+按时间正序排列)。指定历史日期时返回该日全量成交。

```
GET /api/trade?code=000001
GET /api/trade?code=000001&date=20260605
```

响应示例：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "Count": 4190,
    "List": [
      {
        "Time": "2024-11-03T09:25:00Z",
        "Price": 12500,
        "Volume": 100,
        "Status": 0,
        "Number": 5
      }
    ]
  }
}
```

- Status: 0=主动买入, 1=主动卖出, 2=中性
- 价格单位：厘，成交量单位：手

### 5.2 获取历史分时成交(全量)

`GET /api/trade-history`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| date | string | 是 | 交易日期(YYYYMMDD) |

返回指定日期的全量分时成交，自动分页获取并按时间正序排列。

```
GET /api/trade-history?code=000001&date=20260605
```

### 5.3 获取全天分时成交

`GET /api/minute-trade-all`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| date | string | 否 | 交易日期(YYYYMMDD)，默认当天 |

功能与 `/api/trade` 相同，返回全量分时成交。

### 5.4 获取上市以来分时成交

`GET /api/trade-history/full`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| before | string | 否 | 截止日期(YYYYMMDD)，默认今日 |
| limit | int | 否 | 截取最近N条 |

返回上市以来的全部历史分时成交。数据量极大，建议配合 `before` 和 `limit` 使用。

---

## 6. 代码表

### 6.1 获取证券代码列表

`GET /api/codes`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| exchange | string | 否 | 交易所: sh/sz/bj/all，默认 all |

### 6.2 获取股票代码列表

`GET /api/stock-codes`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| limit | int | 否 | 返回条数限制 |
| prefix | bool | 否 | 是否包含交易所前缀，默认 true |

### 6.3 获取ETF代码列表

`GET /api/etf-codes`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| limit | int | 否 | 返回条数限制 |
| prefix | bool | 否 | 是否包含交易所前缀，默认 true |

### 6.4 获取ETF列表

`GET /api/etf`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| exchange | string | 否 | 交易所: sh/sz/all，默认 all |
| limit | int | 否 | 返回条数限制 |

### 6.5 搜索股票

`GET /api/search`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| keyword | string | 是 | 搜索关键词(代码或名称) |

支持代码和名称模糊搜索，最多返回50条结果。

```
GET /api/search?keyword=平安
GET /api/search?keyword=000001
```

### 6.6 获取市场证券数量

`GET /api/market-count`

返回上交所、深交所、北交所的证券数量统计。

---

## 7. 综合信息

### 7.1 获取股票综合信息

`GET /api/stock-info`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |

一次性返回五档行情 + 最近30条日K线 + 今日分时数据，适合快速获取股票概览。

---

## 8. 除权除息

### 8.1 获取除权除息数据

`GET /api/gbbq`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |

返回该股票的除权除息列表。

---

## 9. 财务信息

### 9.1 获取财务信息

`GET /api/finance`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |

返回标的财务/基本面信息（流通股本/总股本/行业/地域/股东户数/财务指标）。

---

## 10. 板块

### 10.1 获取板块列表

`GET /api/blocks`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| type | string | 否 | 板块类型，默认 concept |

### 10.2 获取板块成分

`GET /api/block-members`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| name | string | 是 | 板块名称 |

---

## 11. 集合竞价

### 11.1 获取集合竞价

`GET /api/call-auction`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |

---

## 12. 工作日

### 12.1 查询交易日信息

`GET /api/workday`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| date | string | 否 | 查询日期(YYYYMMDD)，默认当天 |
| count | int | 否 | 前后交易日数量，1-30，默认1 |

### 12.2 获取交易日范围

`GET /api/workday/range`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| start | string | 是 | 起始日期(YYYYMMDD) |
| end | string | 是 | 结束日期(YYYYMMDD) |

---

## 13. 收益

### 13.1 计算收益区间

`GET /api/income`

| 参数 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| code | string | 是 | 股票代码 |
| start_date | string | 是 | 基准日期(YYYYMMDD) |
| days | string | 否 | 天数偏移(逗号分隔)，默认 5,10,20,60,120 |

以基准日收盘价为基准，计算若干交易日后的收益情况。

---

## 14. 任务管理

### 14.1 创建K线入库任务

`POST /api/tasks/pull-kline`

请求体：
```json
{
  "codes": ["000001", "600519"],
  "tables": ["day", "week", "month"],
  "dir": "data/database/kline",
  "limit": 4,
  "start_date": "2020-01-01"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| codes | array | 否 | 股票代码数组，默认遍历全部A股 |
| tables | array | 否 | K线类型，默认 ["day"] |
| dir | string | 否 | 存储目录，默认 data/database/kline |
| limit | int | 否 | 并发协程数，默认1 |
| start_date | string | 否 | 起始日期阈值 |

K线类型: `minute`, `5minute`, `15minute`, `30minute`, `hour`, `day`, `week`, `month`, `quarter`, `year`

### 14.2 创建分时成交入库任务

`POST /api/tasks/pull-trade`

请求体：
```json
{
  "code": "000001",
  "dir": "data/database/trade",
  "start_year": 2015,
  "end_year": 2025
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| code | string | 是 | 股票代码 |
| dir | string | 否 | 输出目录，默认 data/database/trade |
| start_year | int | 否 | 起始年份，默认2000 |
| end_year | int | 否 | 结束年份，默认当年 |

### 14.3 查询与控制任务

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/tasks` | GET | 列出所有任务 |
| `/api/tasks/{id}` | GET | 查询任务详情 |
| `/api/tasks/{id}/cancel` | POST | 取消任务 |

任务状态: `running` / `success` / `failed` / `cancelled`

---

## 15. 系统

### 15.1 获取服务状态

`GET /api/server-status`

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "running",
    "connected": true,
    "version": "1.0.0",
    "uptime": "5m30s"
  }
}
```

### 15.2 获取接口文档

`GET /api/docs`

返回所有接口的元信息列表。

---

## 数据单位

| 类型 | 单位 | 换算 |
|------|------|------|
| 价格 | 厘 | 元 = 厘 / 1000，如 12500厘 = 12.50元 |
| 成交量 | 手 | 股 = 手 × 100，如 1235手 = 123500股 |
| 成交额 | 厘 | 元 = 厘 / 1000 |
| 挂单量 | 股 | 直接使用 |

---

## 错误码

| code | message | 说明 |
|------|---------|------|
| 0 | success | 请求成功 |
| -1 | 股票代码不能为空 | 缺少必填参数 code |
| -1 | 获取行情失败: xxx | 数据获取失败 |
| -1 | 获取K线失败: xxx | K线数据获取失败 |
| -1 | 未找到相关股票 | 搜索无结果 |
| -1 | 搜索关键词不能为空 | 缺少 keyword 参数 |

---

## 使用示例

### Python

```python
import requests

BASE = "http://localhost:8080"

def get_quote(code):
    r = requests.get(f"{BASE}/api/quote", params={"code": code})
    d = r.json()
    return d["data"] if d["code"] == 0 else None

def get_kline(code, type="day", count=100):
    r = requests.get(f"{BASE}/api/kline", params={"code": code, "type": type, "count": count})
    d = r.json()
    return d["data"]["List"] if d["code"] == 0 else None

def get_trade_all(code, date=None):
    params = {"code": code}
    if date:
        params["date"] = date
    r = requests.get(f"{BASE}/api/trade", params=params)
    d = r.json()
    return d["data"] if d["code"] == 0 else None

def search(keyword):
    r = requests.get(f"{BASE}/api/search", params={"keyword": keyword})
    d = r.json()
    return d["data"] if d["code"] == 0 else None

# 使用
quote = get_quote("000001")
print(f"最新价: {quote[0]['K']['Close'] / 1000}元")

klines = get_kline("000001", "day", 30)
print(f"获取{len(klines)}条K线")

trades = get_trade_all("000001")
print(f"今日成交{trades['Count']}条")
```

### cURL

```bash
# 五档行情
curl "http://localhost:8080/api/quote?code=000001"

# 日K线
curl "http://localhost:8080/api/kline?code=000001&type=day&count=30"

# 全量K线(最近100条)
curl "http://localhost:8080/api/kline-all?code=000001&type=day&limit=100"

# 分时数据
curl "http://localhost:8080/api/minute?code=000001"

# 全量分时成交
curl "http://localhost:8080/api/trade?code=000001"

# 历史分时成交(全量)
curl "http://localhost:8080/api/trade-history?code=000001&date=20260605"

# 搜索
curl "http://localhost:8080/api/search?keyword=平安"

# 批量行情
curl -X POST http://localhost:8080/api/batch-quote \
  -H "Content-Type: application/json" \
  -d '{"codes":["000001","600519"]}'

# 指数K线
curl "http://localhost:8080/api/index?code=sh000001&type=day&count=10"

# 财务信息
curl "http://localhost:8080/api/finance?code=000001"

# 服务状态
curl "http://localhost:8080/api/server-status"
```

---

## 部署

### 本地部署

```bash
./deploy.sh                # 编译+启动
TDX_PORT=9090 ./deploy.sh # 指定端口
./deploy.sh stop           # 停止
./deploy.sh status         # 状态
```

### Docker 部署

```bash
./deploy.sh docker        # 构建镜像+启动
docker compose logs -f    # 查看日志
./deploy.sh stop          # 停止
```

### Makefile

```bash
make            # 编译
make run        # 编译+运行
make clean      # 清理
make docker     # 构建Docker镜像
make docker-up  # Docker Compose启动
make cross      # 交叉编译
```

---

## 性能建议

1. **批量请求**: 使用 `/api/batch-quote` 代替多次单个请求
2. **缓存**: 对不常变化的数据(如股票列表)做本地缓存
3. **限流**: 避免频繁请求，建议间隔 >= 3秒
4. **全量数据**: `/api/kline-all` 和 `/api/trade-history/full` 数据量大，配合 `limit` 参数控制
5. **超时设置**: 全量K线接口(上市时间长的标的)建议设置 >= 10秒超时
