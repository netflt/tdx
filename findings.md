# 盘后选股系统 - 发现记录

## 可用数据能力

### 核心数据（盘后可用）
1. **日K线全量**: GetKlineDayAll(code) → OHLCV + 成交额
2. **前复权日K**: Gbbq.QFQ(code, ks) → 对齐通达信
3. **财务信息**: GetFinanceInfo(exchange, code) → 30+字段
4. **行业归属**: GetTdxHy() → 通达信行业 + 申万行业
5. **概念板块**: GetBlockData(BlockFileGN) → 板块+成分股
6. **盘后统计**: GetTdxStat() → PE_TTM/股息率/连涨/区间涨跌幅
7. **资金流向**: GetTdxStat2() → 成交额/IPO价/52周高低
8. **除权除息**: Gbbq → 复权因子计算
9. **交易日历**: Workday → 判断交易日

### 内置技术指标
- MA(n), EMA(n), MACD(), RSI(n), BOLL(n), ATR(n), VWAP()
- HHV(n), LLV(n), REF(n)

### 数据获取模式
- 全市场扫描：遍历 codes 获取股票列表，并发拉取日K
- 单股深度：拉取多周期K线 + 分时成交 + 财务
- 增量更新：基于 workday 判断最新交易日

### 性能考量
- 全市场 ~5000 只股票
- 单只日K拉取 ~100ms（网络延迟）
- 全市场串行 ~8分钟，并发(20) ~25秒
- 技术指标计算在本地，极快
