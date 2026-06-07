package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/injoyai/conv"
	"github.com/injoyai/logs"
	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/extend"
	"github.com/injoyai/tdx/protocol"
)

// ========================================
// 全局状态
// ========================================

var (
	client  *tdx.Client
	codes   tdx.ICodes
	gbbq    *tdx.Gbbq
	workday *tdx.Workday
	startAt time.Time
	dbType  string
)

// Config 服务配置
type Config struct {
	Port     int    // 服务端口
	Dir      string // 数据目录
	NoWeb    bool   // 禁用Web测试页面
	DBType   string // 数据库类型: mysql/sqlite, 默认mysql
	DBDSN    string // 数据库DSN(mysql时必填)
}

// Run 启动服务
func Run(cfg Config) error {
	startAt = time.Now()

	// 1. 连接通达信服务器
	var err error
	client, err = tdx.DialWith(tdx.NewHostDial(tdx.Hosts))
	if err != nil {
		return fmt.Errorf("连接通达信服务器失败: %w", err)
	}

	// 2. 根据数据库类型初始化
	dbType = cfg.DBType
	if dbType == "" {
		dbType = "mysql"
	}

	fmt.Printf("  数据库: %s\n", dbType)

	switch dbType {
	case "mysql":
		dsn := cfg.DBDSN
		if dsn == "" {
			return fmt.Errorf("MySQL模式需要配置 DB_DSN (或 DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME)")
		}
		// 代码表(MySQL)
		codes, err = tdx.NewCodesMysql(dsn, tdx.WithCodesClient(client))
		if err != nil {
			return fmt.Errorf("初始化代码表(MySQL)失败: %w", err)
		}
		tdx.DefaultCodes = codes

		// 后台初始化除权除息和工作日
		go func() {
			if g, err := tdx.NewGbbq(tdx.WithGbbqClient(client)); err != nil {
				logs.Errorf("初始化除权除息失败: %v", err)
			} else {
				gbbq = g
			}
		}()
		go func() {
			if w, err := tdx.NewWorkdayMysql(dsn, tdx.WithWorkdayClient(client)); err != nil {
				logs.Errorf("初始化工作日(MySQL)失败: %v", err)
			} else {
				workday = w
			}
		}()

	default:
		// SQLite模式
		codes, err = tdx.NewCodesSqlite(tdx.WithCodesClient(client))
		if err != nil {
			return fmt.Errorf("初始化代码表失败: %w", err)
		}
		tdx.DefaultCodes = codes

		go func() {
			if g, err := tdx.NewGbbq(tdx.WithGbbqClient(client)); err != nil {
				logs.Errorf("初始化除权除息失败: %v", err)
			} else {
				gbbq = g
			}
		}()
		go func() {
			if w, err := tdx.NewWorkday(tdx.WithWorkdayClient(client)); err != nil {
				logs.Errorf("初始化工作日失败: %v", err)
			} else {
				workday = w
			}
		}()
	}

	// 3. 注册路由
	registerRoutes()

	// 4. 启动HTTP服务
	addr := fmt.Sprintf(":%d", cfg.Port)
	handler := buildHandler()

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  TDX 股票数据 API 服务已启动")
	fmt.Printf("  地址: http://localhost:%d\n", cfg.Port)
	fmt.Printf("  文档: http://localhost:%d/api/docs\n", cfg.Port)
	fmt.Printf("  测试: http://localhost:%d/\n", cfg.Port)
	fmt.Printf("  数据库: %s\n", dbType)
	fmt.Println("========================================")
	fmt.Println()

	server := &http.Server{Addr: addr, Handler: handler}

	// 优雅关闭
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		fmt.Println("\n正在关闭服务...")
		server.Shutdown(context.Background())
		client.Close()
	}()

	return server.ListenAndServe()
}

// ========================================
// 路由注册
// ========================================

func registerRoutes() {
	// --- 基础 ---
	GET("/api/server-status", handleServerStatus)
	GET("/api/docs", handleDocs)

	// --- 行情 ---
	GET("/api/quote", handleQuote)
	POST("/api/batch-quote", handleBatchQuote)

	// --- K线 ---
	GET("/api/kline", handleKline)
	GET("/api/kline-all", handleKlineAll)
	GET("/api/kline-all/tdx", handleKlineAllTDX)
	GET("/api/kline-all/ths", handleKlineAllTHS)
	GET("/api/kline-history", handleKlineHistory)

	// --- 指数 ---
	GET("/api/index", handleIndex)
	GET("/api/index/all", handleIndexAll)

	// --- 分时 ---
	GET("/api/minute", handleMinute)

	// --- 分时成交 ---
	GET("/api/trade", handleTrade)
	GET("/api/trade-history", handleTradeHistory)
	GET("/api/minute-trade-all", handleMinuteTradeAll)
	GET("/api/trade-history/full", handleTradeHistoryFull)

	// --- 代码表 ---
	GET("/api/codes", handleCodes)
	GET("/api/stock-codes", handleStockCodes)
	GET("/api/etf-codes", handleETFCodes)
	GET("/api/etf", handleETFList)
	GET("/api/search", handleSearch)
	GET("/api/market-count", handleMarketCount)

	// --- 综合信息 ---
	GET("/api/stock-info", handleStockInfo)

	// --- 除权除息 ---
	GET("/api/gbbq", handleGbbq)

	// --- 财务 ---
	GET("/api/finance", handleFinance)

	// --- 板块 ---
	GET("/api/blocks", handleBlocks)
	GET("/api/block-members", handleBlockMembers)

	// --- 集合竞价 ---
	GET("/api/call-auction", handleCallAuction)

	// --- 工作日 ---
	GET("/api/workday", handleWorkday)
	GET("/api/workday/range", handleWorkdayRange)

	// --- 收益 ---
	GET("/api/income", handleIncome)

	// --- 任务 ---
	POST("/api/tasks/pull-kline", handlePullKline)
	POST("/api/tasks/pull-trade", handlePullTrade)
	GET("/api/tasks", handleTasks)
	GET("/api/tasks/{id}", handleTaskDetail)
	POST("/api/tasks/{id}/cancel", handleTaskCancel)
}

// ========================================
// 接口实现: 基础
// ========================================

func handleServerStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, OK(map[string]any{
		"status":    "running",
		"connected": client != nil,
		"version":   "1.0.0",
		"uptime":    time.Since(startAt).Round(time.Second).String(),
		"database":  dbType,
	}))
}

func handleDocs(w http.ResponseWriter, r *http.Request) {
	type apiDoc struct {
		Method  string `json:"method"`
		Path    string `json:"path"`
		Desc    string `json:"desc"`
		Params  string `json:"params,omitempty"`
	}
	docs := []apiDoc{
		{Method: "GET", Path: "/api/server-status", Desc: "服务状态"},
		{Method: "GET", Path: "/api/quote", Desc: "五档行情", Params: "code(必填,逗号分隔多只)"},
		{Method: "POST", Path: "/api/batch-quote", Desc: "批量行情", Params: "body: {codes:[]}"},
		{Method: "GET", Path: "/api/kline", Desc: "K线数据", Params: "code,type=day,count=100"},
		{Method: "GET", Path: "/api/kline-all", Desc: "全量K线", Params: "code,type=day,limit"},
		{Method: "GET", Path: "/api/kline-all/tdx", Desc: "通达信全量K线", Params: "code,type=day,limit"},
		{Method: "GET", Path: "/api/kline-all/ths", Desc: "同花顺前复权K线", Params: "code,type=day,limit"},
		{Method: "GET", Path: "/api/kline-history", Desc: "历史K线", Params: "code,type=day,start_date,end_date,limit=100"},
		{Method: "GET", Path: "/api/index", Desc: "指数K线", Params: "code,type=day,count=100"},
		{Method: "GET", Path: "/api/index/all", Desc: "指数全量K线", Params: "code,type=day,limit"},
		{Method: "GET", Path: "/api/minute", Desc: "分时数据", Params: "code,date"},
		{Method: "GET", Path: "/api/trade", Desc: "分时成交", Params: "code,date"},
		{Method: "GET", Path: "/api/trade-history", Desc: "历史分时成交(全量)", Params: "code,date"},
		{Method: "GET", Path: "/api/minute-trade-all", Desc: "全天分时成交", Params: "code,date"},
		{Method: "GET", Path: "/api/trade-history/full", Desc: "上市至今分时成交", Params: "code,before,limit"},
		{Method: "GET", Path: "/api/codes", Desc: "证券代码列表", Params: "exchange=all"},
		{Method: "GET", Path: "/api/stock-codes", Desc: "股票代码列表", Params: "limit,prefix=true"},
		{Method: "GET", Path: "/api/etf-codes", Desc: "ETF代码列表", Params: "limit,prefix=true"},
		{Method: "GET", Path: "/api/etf", Desc: "ETF列表", Params: "exchange=all,limit"},
		{Method: "GET", Path: "/api/search", Desc: "搜索股票", Params: "keyword(必填)"},
		{Method: "GET", Path: "/api/market-count", Desc: "市场证券数量"},
		{Method: "GET", Path: "/api/stock-info", Desc: "股票综合信息", Params: "code(必填)"},
		{Method: "GET", Path: "/api/gbbq", Desc: "除权除息", Params: "code(必填)"},
		{Method: "GET", Path: "/api/finance", Desc: "财务信息", Params: "code(必填)"},
		{Method: "GET", Path: "/api/blocks", Desc: "板块列表", Params: "type=concept"},
		{Method: "GET", Path: "/api/block-members", Desc: "板块成分", Params: "name(必填)"},
		{Method: "GET", Path: "/api/call-auction", Desc: "集合竞价", Params: "code(必填)"},
		{Method: "GET", Path: "/api/workday", Desc: "交易日信息", Params: "date,count=1"},
		{Method: "GET", Path: "/api/workday/range", Desc: "交易日范围", Params: "start(必填),end(必填)"},
		{Method: "GET", Path: "/api/income", Desc: "收益区间", Params: "code(必填),start_date(必填),days=5,10,20,60,120"},
		{Method: "POST", Path: "/api/tasks/pull-kline", Desc: "创建K线入库任务", Params: "body: {codes,tables,dir,limit,start_date}"},
		{Method: "POST", Path: "/api/tasks/pull-trade", Desc: "创建分时成交入库任务", Params: "body: {code,dir,start_year,end_year}"},
		{Method: "GET", Path: "/api/tasks", Desc: "任务列表"},
		{Method: "GET", Path: "/api/tasks/{id}", Desc: "任务详情"},
		{Method: "POST", Path: "/api/tasks/{id}/cancel", Desc: "取消任务"},
	}
	writeJSON(w, OK(docs))
}

// ========================================
// 接口实现: 行情
// ========================================

func handleQuote(w http.ResponseWriter, r *http.Request) {
	codeStr := queryStr(r, "code")
	if codeStr == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	codes := queryCodes(r, "code")
	resp, err := client.GetQuote(codes...)
	if err != nil {
		writeJSON(w, Failf("获取行情失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

func handleBatchQuote(w http.ResponseWriter, r *http.Request) {
	var req struct{ Codes []string `json:"codes"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, Failf("请求解析失败: %v", err))
		return
	}
	if len(req.Codes) == 0 {
		writeJSON(w, Fail("股票代码列表不能为空"))
		return
	}
	resp, err := client.GetQuote(req.Codes...)
	if err != nil {
		writeJSON(w, Failf("获取行情失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

// ========================================
// 接口实现: K线
// ========================================

func klineType(typeStr string) uint8 {
	switch typeStr {
	case "minute1", "minute":
		return protocol.TypeKlineMinute
	case "minute5", "5minute":
		return protocol.TypeKline5Minute
	case "minute15", "15minute":
		return protocol.TypeKline15Minute
	case "minute30", "30minute":
		return protocol.TypeKline30Minute
	case "hour", "minute60", "60minute":
		return protocol.TypeKlineHour
	case "day":
		return protocol.TypeKlineDay
	case "week":
		return protocol.TypeKlineWeek
	case "month":
		return protocol.TypeKlineMonth
	case "quarter":
		return protocol.TypeKlineQuarter
	case "year":
		return protocol.TypeKlineYear
	default:
		return protocol.TypeKlineDay
	}
}

func handleKline(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	typeStr := queryStr(r, "type", "day")
	count := uint16(queryInt(r, "count", 100))

	resp, err := client.GetKline(klineType(typeStr), code, 0, count)
	if err != nil {
		writeJSON(w, Failf("获取K线失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

func handleKlineAll(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	typeStr := queryStr(r, "type", "day")
	limit := queryInt(r, "limit", 0)

	resp, err := client.GetKlineAll(klineType(typeStr), code)
	if err != nil {
		writeJSON(w, Failf("获取K线失败: %v", err))
		return
	}

	list := resp.List
	if limit > 0 && limit < len(list) {
		list = list[len(list)-limit:]
	}
	writeJSON(w, OK(map[string]any{
		"count": len(list),
		"list":  list,
	}))
}

func handleKlineAllTDX(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	typeStr := queryStr(r, "type", "day")
	limit := queryInt(r, "limit", 0)

	resp, err := client.GetKlineAll(klineType(typeStr), code)
	if err != nil {
		writeJSON(w, Failf("获取K线失败: %v", err))
		return
	}

	list := resp.List
	if limit > 0 && limit < len(list) {
		list = list[len(list)-limit:]
	}
	writeJSON(w, OK(map[string]any{
		"count": len(list),
		"list":  list,
		"meta": map[string]any{
			"source":     "tdx",
			"type":       typeStr,
			"batch_limit": 800,
		},
	}))
}

func handleKlineAllTHS(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	typeStr := queryStr(r, "type", "day")
	limit := queryInt(r, "limit", 0)

	if typeStr != "day" && typeStr != "week" && typeStr != "month" {
		writeJSON(w, Fail("同花顺接口仅支持 day/week/month"))
		return
	}

	ks, err := extend.GetTHSDayKline(code, extend.THS_QFQ)
	if err != nil {
		writeJSON(w, Failf("获取同花顺K线失败: %v", err))
		return
	}

	list := ks
	if typeStr == "week" {
		list = list.Merge(5)
	} else if typeStr == "month" {
		list = list.Merge(22)
	}

	if limit > 0 && limit < len(list) {
		list = list[len(list)-limit:]
	}
	writeJSON(w, OK(map[string]any{
		"count": len(list),
		"list":  list,
		"meta": map[string]any{
			"source": "ths",
			"type":   typeStr,
		},
	}))
}

func handleKlineHistory(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	typeStr := queryStr(r, "type", "day")
	limit := uint16(queryInt(r, "limit", 100))
	if limit > 800 {
		limit = 800
	}

	resp, err := client.GetKline(klineType(typeStr), code, 0, limit)
	if err != nil {
		writeJSON(w, Failf("获取K线失败: %v", err))
		return
	}

	startDate := queryStr(r, "start_date")
	endDate := queryStr(r, "end_date")

	list := resp.List
	if startDate != "" || endDate != "" {
		filtered := make(protocol.Klines, 0, len(list))
		for _, k := range list {
			d := k.Time.Format("20060102")
			if startDate != "" && d < startDate {
				continue
			}
			if endDate != "" && d > endDate {
				continue
			}
			filtered = append(filtered, k)
		}
		list = filtered
	}

	writeJSON(w, OK(map[string]any{
		"count": len(list),
		"list":  list,
	}))
}

// ========================================
// 接口实现: 指数
// ========================================

func handleIndex(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("指数代码不能为空"))
		return
	}
	typeStr := queryStr(r, "type", "day")
	count := uint16(queryInt(r, "count", 100))

	resp, err := client.GetIndex(klineType(typeStr), code, 0, count)
	if err != nil {
		writeJSON(w, Failf("获取指数失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

func handleIndexAll(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("指数代码不能为空"))
		return
	}
	typeStr := queryStr(r, "type", "day")
	limit := queryInt(r, "limit", 0)

	resp, err := client.GetIndexAll(klineType(typeStr), code)
	if err != nil {
		writeJSON(w, Failf("获取指数失败: %v", err))
		return
	}

	list := resp.List
	if limit > 0 && limit < len(list) {
		list = list[len(list)-limit:]
	}
	writeJSON(w, OK(map[string]any{
		"count": len(list),
		"list":  list,
	}))
}

// ========================================
// 接口实现: 分时
// ========================================

func handleMinute(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	date := queryStr(r, "date")

	var resp *protocol.MinuteResp
	var err error
	if date != "" {
		resp, err = client.GetHistoryMinute(date, code)
	} else {
		resp, err = client.GetMinute(code)
	}
	if err != nil {
		writeJSON(w, Failf("获取分时数据失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

// ========================================
// 接口实现: 分时成交
// ========================================

func handleTrade(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	date := queryStr(r, "date")

	var resp *protocol.TradeResp
	var err error
	if date != "" && date != time.Now().Format("20060102") {
		resp, err = client.GetHistoryMinuteTradeDay(date, code)
	} else {
		resp, err = client.GetMinuteTradeAll(code)
	}
	if err != nil {
		writeJSON(w, Failf("获取分时成交失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

func handleTradeHistory(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	date := queryStr(r, "date")
	if date == "" {
		writeJSON(w, Fail("日期不能为空"))
		return
	}

	resp, err := client.GetHistoryMinuteTradeDay(date, code)
	if err != nil {
		writeJSON(w, Failf("获取历史分时成交失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

func handleMinuteTradeAll(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	date := queryStr(r, "date")

	var resp *protocol.TradeResp
	var err error
	if date != "" && date != time.Now().Format("20060102") {
		resp, err = client.GetHistoryMinuteTradeDay(date, code)
	} else {
		resp, err = client.GetMinuteTradeAll(code)
	}
	if err != nil {
		writeJSON(w, Failf("获取分时成交失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

func handleTradeHistoryFull(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	limit := queryInt(r, "limit", 0)
	beforeStr := queryStr(r, "before")

	var before time.Time
	if beforeStr != "" {
		before = parseDate(beforeStr)
	}

	if workday == nil {
		writeJSON(w, Fail("工作日管理器未初始化"))
		return
	}

	var trades protocol.Trades
	var err error
	if !before.IsZero() {
		trades, err = client.GetHistoryTradeBefore(code, workday, before)
	} else {
		trades, err = client.GetHistoryTradeFull(code, workday)
	}
	if err != nil {
		writeJSON(w, Failf("获取历史分时成交失败: %v", err))
		return
	}

	if limit > 0 && limit < len(trades) {
		trades = trades[len(trades)-limit:]
	}
	writeJSON(w, OK(map[string]any{
		"count": len(trades),
		"list":  trades,
	}))
}

// ========================================
// 接口实现: 代码表
// ========================================

func handleCodes(w http.ResponseWriter, r *http.Request) {
	exchange := queryStr(r, "exchange", "all")

	type codeItem struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		Exchange string `json:"exchange"`
	}

	var items []codeItem
	var shCount, szCount, bjCount int

	for k, v := range codes.Iter() {
		fullCode := v.FullCode()
		ex := ""
		if strings.HasPrefix(fullCode, "sh") {
			ex = "sh"
			shCount++
		} else if strings.HasPrefix(fullCode, "sz") {
			ex = "sz"
			szCount++
		} else if strings.HasPrefix(fullCode, "bj") {
			ex = "bj"
			bjCount++
		}

		if exchange != "all" && exchange != ex {
			continue
		}
		items = append(items, codeItem{
			Code:     k,
			Name:     v.Name,
			Exchange: ex,
		})
	}

	writeJSON(w, OK(map[string]any{
		"total": len(items),
		"exchanges": map[string]int{
			"sh": shCount, "sz": szCount, "bj": bjCount,
		},
		"codes": items,
	}))
}

func handleStockCodes(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 0)
	prefix := queryBool(r, "prefix", true)

	stocks := codes.GetStockCodes()
	if limit > 0 && limit < len(stocks) {
		stocks = stocks[:limit]
	}
	if !prefix {
		for i, c := range stocks {
			stocks[i] = strings.TrimPrefix(strings.TrimPrefix(c, "sh"), "sz")
		}
	}
	writeJSON(w, OK(map[string]any{
		"count": len(stocks),
		"list":  stocks,
	}))
}

func handleETFCodes(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 0)
	prefix := queryBool(r, "prefix", true)

	etfs := codes.GetETFCodes()
	if limit > 0 && limit < len(etfs) {
		etfs = etfs[:limit]
	}
	if !prefix {
		for i, c := range etfs {
			etfs[i] = strings.TrimPrefix(strings.TrimPrefix(c, "sh"), "sz")
		}
	}
	writeJSON(w, OK(map[string]any{
		"count": len(etfs),
		"list":  etfs,
	}))
}

func handleETFList(w http.ResponseWriter, r *http.Request) {
	exchange := queryStr(r, "exchange", "all")
	limit := queryInt(r, "limit", 0)

	type etfItem struct {
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		Exchange string  `json:"exchange"`
	}
	items := []etfItem{}
	for _, v := range codes.GetETFs() {
		fullCode := v.FullCode()
		ex := ""
		if strings.HasPrefix(fullCode, "sh") {
			ex = "sh"
		} else if strings.HasPrefix(fullCode, "sz") {
			ex = "sz"
		}
		if exchange != "all" && exchange != ex {
			continue
		}
		items = append(items, etfItem{
			Code:     fullCode,
			Name:     v.Name,
			Exchange: ex,
		})
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	writeJSON(w, OK(map[string]any{
		"total": len(items),
		"list":  items,
	}))
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	keyword := queryStr(r, "keyword")
	if keyword == "" {
		writeJSON(w, Fail("搜索关键词不能为空"))
		return
	}

	type searchItem struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	items := []searchItem{}
	count := 0
	for k, v := range codes.Iter() {
		if count >= 50 {
			break
		}
		if strings.Contains(k, keyword) || strings.Contains(v.Name, keyword) {
			items = append(items, searchItem{Code: k, Name: v.Name})
			count++
		}
	}
	if len(items) == 0 {
		writeJSON(w, Fail("未找到相关股票"))
		return
	}
	writeJSON(w, OK(items))
}

func handleMarketCount(w http.ResponseWriter, r *http.Request) {
	sh, _ := client.GetCount(protocol.ExchangeSH)
	sz, _ := client.GetCount(protocol.ExchangeSZ)
	bj, _ := client.GetCount(protocol.ExchangeBJ)

	type exCount struct {
		Exchange string `json:"exchange"`
		Count    uint16 `json:"count"`
	}
	writeJSON(w, OK(map[string]any{
		"total": sh.Count + sz.Count + bj.Count,
		"exchanges": []exCount{
			{Exchange: "sh", Count: sh.Count},
			{Exchange: "sz", Count: sz.Count},
			{Exchange: "bj", Count: bj.Count},
		},
	}))
}

// ========================================
// 接口实现: 综合信息
// ========================================

func handleStockInfo(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}

	result := make(map[string]any)

	// 五档行情
	if quote, err := client.GetQuote(code); err == nil && len(quote) > 0 {
		result["quote"] = quote[0]
	}

	// 日K线(最近30条)
	if kline, err := client.GetKlineDay(code, 0, 30); err == nil {
		result["kline_day"] = kline
	}

	// 分时
	if minute, err := client.GetMinute(code); err == nil {
		result["minute"] = minute
	}

	writeJSON(w, OK(result))
}

// ========================================
// 接口实现: 除权除息
// ========================================

func handleGbbq(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	if gbbq == nil {
		writeJSON(w, Fail("除权除息管理器未初始化"))
		return
	}
	xrxds := gbbq.GetXRXDs(code)
	writeJSON(w, OK(xrxds))
}

// ========================================
// 接口实现: 财务
// ========================================

func handleFinance(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	code = protocol.AddPrefix(code)
	var exchange protocol.Exchange
	if strings.HasPrefix(code, "sh") {
		exchange = protocol.ExchangeSH
	} else {
		exchange = protocol.ExchangeSZ
	}
	number := code[2:]

	info, err := client.GetFinanceInfo(exchange, number)
	if err != nil {
		writeJSON(w, Failf("获取财务信息失败: %v", err))
		return
	}
	writeJSON(w, OK(info))
}

// ========================================
// 接口实现: 板块
// ========================================

func handleBlocks(w http.ResponseWriter, r *http.Request) {
	_ = queryStr(r, "type", "concept")

	data, err := client.GetTdxHy()
	if err != nil {
		writeJSON(w, Failf("获取板块数据失败: %v", err))
		return
	}
	writeJSON(w, OK(map[string]any{
		"count": len(data),
		"list":  data,
	}))
}

func handleBlockMembers(w http.ResponseWriter, r *http.Request) {
	name := queryStr(r, "name")
	if name == "" {
		writeJSON(w, Fail("板块名称不能为空"))
		return
	}

	data, err := client.GetBlockData(name)
	if err != nil {
		writeJSON(w, Failf("获取板块成分失败: %v", err))
		return
	}
	writeJSON(w, OK(map[string]any{
		"count": len(data),
		"list":  data,
	}))
}

// ========================================
// 接口实现: 集合竞价
// ========================================

func handleCallAuction(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	resp, err := client.GetCallAuction(code)
	if err != nil {
		writeJSON(w, Failf("获取集合竞价失败: %v", err))
		return
	}
	writeJSON(w, OK(resp))
}

// ========================================
// 接口实现: 工作日
// ========================================

func handleWorkday(w http.ResponseWriter, r *http.Request) {
	if workday == nil {
		writeJSON(w, Fail("工作日管理器未初始化"))
		return
	}
	dateStr := queryStr(r, "date")
	count := queryInt(r, "count", 1)
	if count < 1 {
		count = 1
	}
	if count > 30 {
		count = 30
	}

	var t time.Time
	if dateStr != "" {
		t = parseDate(dateStr)
	} else {
		t = time.Now()
	}

	type dateInfo struct {
		ISO     string `json:"iso"`
		Numeric string `json:"numeric"`
	}

	isWorkday := workday.TodayIs()

	next := make([]dateInfo, 0, count)
	prev := make([]dateInfo, 0, count)

	// 简单实现: 基于workday判断
	cur := t
	for i := 0; i < count*7 && len(next) < count; i++ {
		cur = cur.AddDate(0, 0, 1)
		if workday.TodayIs() {
			next = append(next, dateInfo{ISO: cur.Format("2006-01-02"), Numeric: cur.Format("20060102")})
		}
	}
	cur = t
	for i := 0; i < count*7 && len(prev) < count; i++ {
		cur = cur.AddDate(0, 0, -1)
		if workday.TodayIs() {
			prev = append(prev, dateInfo{ISO: cur.Format("2006-01-02"), Numeric: cur.Format("20060102")})
		}
	}

	writeJSON(w, OK(map[string]any{
		"date":       dateInfo{ISO: t.Format("2006-01-02"), Numeric: t.Format("20060102")},
		"is_workday": isWorkday,
		"next":       next,
		"previous":   prev,
	}))
}

func handleWorkdayRange(w http.ResponseWriter, r *http.Request) {
	if workday == nil {
		writeJSON(w, Fail("工作日管理器未初始化"))
		return
	}
	startStr := queryStr(r, "start")
	endStr := queryStr(r, "end")
	if startStr == "" || endStr == "" {
		writeJSON(w, Fail("start和end参数不能为空"))
		return
	}

	start := parseDate(startStr)
	end := parseDate(endStr)

	type dateInfo struct {
		ISO     string `json:"iso"`
		Numeric string `json:"numeric"`
	}
	dates := []dateInfo{}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, dateInfo{ISO: d.Format("2006-01-02"), Numeric: d.Format("20060102")})
	}
	writeJSON(w, OK(map[string]any{
		"count": len(dates),
		"list":  dates,
	}))
}

// ========================================
// 接口实现: 收益
// ========================================

func handleIncome(w http.ResponseWriter, r *http.Request) {
	code := queryStr(r, "code")
	if code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	startDateStr := queryStr(r, "start_date")
	if startDateStr == "" {
		writeJSON(w, Fail("start_date不能为空"))
		return
	}
	daysStr := queryStr(r, "days", "5,10,20,60,120")

	startAt := parseDate(startDateStr)
	days := []int{}
	for _, d := range strings.Split(daysStr, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(d)); err == nil {
			days = append(days, n)
		}
	}

	resp, err := client.GetKlineDayAll(code)
	if err != nil {
		writeJSON(w, Failf("获取K线失败: %v", err))
		return
	}

	incomes := extend.DoIncomes(resp.List, startAt, days...)

	type incomeItem struct {
		Offset   int           `json:"offset"`
		Time     time.Time     `json:"time"`
		Rise     protocol.Price `json:"rise"`
		RiseRate float64       `json:"rise_rate"`
		Source   protocol.K    `json:"source"`
		Current  protocol.K    `json:"current"`
	}

	items := make([]incomeItem, 0, len(incomes))
	for _, v := range incomes {
		items = append(items, incomeItem{
			Offset:   v.Offset,
			Time:     v.Time,
			Rise:     v.Rise(),
			RiseRate: v.RiseRate(),
			Source:   v.Source,
			Current:  v.Current,
		})
	}

	writeJSON(w, OK(map[string]any{
		"count": len(items),
		"list":  items,
	}))
}

// ========================================
// 接口实现: 任务管理
// ========================================

type taskInfo struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	Error     string    `json:"error,omitempty"`
	cancel    context.CancelFunc
}

var (
	taskMap   = make(map[string]*taskInfo)
	taskMutex sync.RWMutex
	taskSeq   atomic.Int64
)

func handlePullKline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Codes      []string `json:"codes"`
		Tables     []string `json:"tables"`
		Dir        string   `json:"dir"`
		Limit      int      `json:"limit"`
		StartDate  string   `json:"start_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, Failf("请求解析失败: %v", err))
		return
	}

	if len(req.Tables) == 0 {
		req.Tables = []string{"day"}
	}
	if req.Dir == "" {
		req.Dir = filepath.Join("data", "database", "kline")
	}
	if req.Limit <= 0 {
		req.Limit = 1
	}

	id := fmt.Sprintf("kline-%d", taskSeq.Add(1))
	ctx, cancel := context.WithCancel(context.Background())

	info := &taskInfo{
		ID:        id,
		Type:      "pull_kline",
		Status:    "running",
		StartedAt: time.Now(),
		cancel:    cancel,
	}
	taskMutex.Lock()
	taskMap[id] = info
	taskMutex.Unlock()

	go func() {
		defer func() {
			taskMutex.Lock()
			info.Status = "success"
			taskMutex.Unlock()
		}()

		cfg := extend.PullKlineConfig{
			Codes:      req.Codes,
			Tables:     req.Tables,
			Dir:        req.Dir,
			Goroutines: req.Limit,
		}
		if req.StartDate != "" {
			cfg.StartAt = parseDate(req.StartDate)
		}

		pull, err := extend.NewPullKline(cfg)
		if err != nil {
			taskMutex.Lock()
			info.Status = "failed"
			info.Error = err.Error()
			taskMutex.Unlock()
			return
		}

		m, err := tdx.NewManage(
			tdx.WithClients(1),
			tdx.WithCodes(codes),
		)
		if err != nil {
			taskMutex.Lock()
			info.Status = "failed"
			info.Error = err.Error()
			taskMutex.Unlock()
			return
		}

		if err := pull.Update(m); err != nil {
			taskMutex.Lock()
			info.Status = "failed"
			info.Error = err.Error()
			taskMutex.Unlock()
			return
		}
	}()

	// 确保ctx被引用
	_ = ctx

	writeJSON(w, OK(map[string]any{"task_id": id}))
}

func handlePullTrade(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code      string `json:"code"`
		Dir       string `json:"dir"`
		StartYear int    `json:"start_year"`
		EndYear   int    `json:"end_year"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, Failf("请求解析失败: %v", err))
		return
	}
	if req.Code == "" {
		writeJSON(w, Fail("股票代码不能为空"))
		return
	}
	if req.Dir == "" {
		req.Dir = filepath.Join("data", "database", "trade")
	}
	if req.StartYear == 0 {
		req.StartYear = 2000
	}
	if req.EndYear == 0 {
		req.EndYear = time.Now().Year()
	}

	id := fmt.Sprintf("trade-%d", taskSeq.Add(1))
	ctx, cancel := context.WithCancel(context.Background())

	info := &taskInfo{
		ID:        id,
		Type:      "pull_trade",
		Status:    "running",
		StartedAt: time.Now(),
		cancel:    cancel,
	}
	taskMutex.Lock()
	taskMap[id] = info
	taskMutex.Unlock()

	go func() {
		defer func() {
			taskMutex.Lock()
			info.Status = "success"
			taskMutex.Unlock()
		}()

		pull := extend.NewPullTrade(req.Dir)
		m, err := tdx.NewManage(
			tdx.WithClients(1),
			tdx.WithCodes(codes),
		)
		if err != nil {
			taskMutex.Lock()
			info.Status = "failed"
			info.Error = err.Error()
			taskMutex.Unlock()
			return
		}

		for year := req.StartYear; year <= req.EndYear; year++ {
			select {
			case <-ctx.Done():
				taskMutex.Lock()
				info.Status = "cancelled"
				taskMutex.Unlock()
				return
			default:
			}
			if err := pull.PullYear(ctx, m, year, req.Code); err != nil {
				taskMutex.Lock()
				info.Status = "failed"
				info.Error = err.Error()
				taskMutex.Unlock()
				return
			}
		}
	}()

	writeJSON(w, OK(map[string]any{"task_id": id}))
}

func handleTasks(w http.ResponseWriter, r *http.Request) {
	taskMutex.RLock()
	defer taskMutex.RUnlock()

	items := make([]*taskInfo, 0, len(taskMap))
	for _, v := range taskMap {
		items = append(items, v)
	}
	writeJSON(w, OK(items))
}

func handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	taskMutex.RLock()
	info, ok := taskMap[id]
	taskMutex.RUnlock()
	if !ok {
		writeJSON(w, Fail("任务不存在"))
		return
	}
	writeJSON(w, OK(info))
}

func handleTaskCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	taskMutex.Lock()
	info, ok := taskMap[id]
	if ok && info.cancel != nil {
		info.cancel()
		info.Status = "cancelled"
	}
	taskMutex.Unlock()
	if !ok {
		writeJSON(w, Fail("任务不存在"))
		return
	}
	writeJSON(w, OK(info))
}

// ========================================
// 辅助函数
// ========================================

func parseDate(s string) time.Time {
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "/", "")
	if len(s) == 8 {
		t, _ := time.ParseInLocation("20060102", s, time.Local)
		return t
	}
	return time.Time{}
}

// 确保 conv.String 可用
var _ = conv.String
