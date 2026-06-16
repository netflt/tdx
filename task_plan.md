# 盘后选股系统 - 任务计划

## 目标
基于 TDX 系统的可用数据，设计并实现一个盘后选股系统，支持多策略筛选、技术指标计算、结果排序和持久化。

## 阶段

### Phase 1: 选股框架设计 [in_progress]
- 设计选股策略接口（Strategy）
- 设计选股上下文（Context），封装数据获取
- 设计选股结果（Result）和排名机制
- 设计选股引擎（Engine），支持多策略组合

### Phase 2: 内置策略实现 [pending]
- 趋势策略：均线多头、MACD金叉、突破新高
- 动量策略：N日涨幅、连涨天数、放量突破
- 价值策略：低PE/低PB、高股息率、ROE筛选
- 技术形态：缩量回踩、布林收口、RSI超卖
- 复合策略：多因子打分、条件组合

### Phase 3: 选股API接口 [pending]
- POST /api/scan - 执行选股
- GET /api/scan/strategies - 列出可用策略
- GET /api/scan/results - 查询历史结果
- 集成到现有 server

### Phase 4: 验证测试 [pending]
- 编译验证
- 实盘数据测试
- 性能测试（全市场扫描耗时）

## 关键决策
- 选股在服务端执行，复用已有的 Client + Gbbq + Codes
- 策略用 Go 代码定义，支持参数化配置
- 全市场扫描并发执行，控制并发数
- 结果存 MySQL，支持历史回溯

## 错误记录
| Error | Attempt | Resolution |
|-------|---------|------------|
