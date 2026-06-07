package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

func main() {
	fmt.Println("=== TDX 实盘数据验证程序 ===")
	fmt.Println()

	// 1. 连接标准行情服务器
	fmt.Println("[1] 连接标准行情服务器...")
	c, err := tdx.DialWith(tdx.NewHostDial(tdx.Hosts))
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer c.Close()
	fmt.Println("连接成功")
	fmt.Println()

	// 2. 初始化代码表
	fmt.Println("[2] 初始化证券代码表...")
	tdx.DefaultCodes, err = tdx.NewCodesSqlite(tdx.WithCodesClient(c))
	if err != nil {
		fmt.Printf("代码表初始化失败: %v\n", err)
		return
	}
	fmt.Println("代码表初始化成功")
	fmt.Println()

	// === 标准行情验证 ===
	verifyQuote(c)
	verifyKlineDay(c)
	verifyKlineMinute(c)
	verifyTrade(c)
	verifyFinanceInfo(c)
	verifyGbbq(c)
	verifyIndexKline(c)

	// === 扩展行情验证 ===
	verifyExHq()

	fmt.Println("\n=== 验证完成 ===")
}

// ==================== 五档行情验证 ====================
func verifyQuote(c *tdx.Client) {
	fmt.Println("========== 五档行情验证 ==========")

	codes := []string{"000001", "600519", "159558", "000300", "920010"}
	resp, err := c.GetQuote(codes...)
	if err != nil {
		fmt.Printf("GetQuote 失败: %v\n", err)
		return
	}

	for _, q := range resp {
		fmt.Printf("\n--- %s%s ---\n", q.Exchange.String(), q.Code)
		fmt.Printf("  昨收: %.3f  今开: %.3f  最高: %.3f  最低: %.3f  最新: %.3f\n",
			q.K.Last.Float64(), q.K.Open.Float64(), q.K.High.Float64(), q.K.Low.Float64(), q.K.Close.Float64())

		// 验证OHLC关系
		if q.K.High.Float64() < q.K.Low.Float64() {
			fmt.Printf("  [错误] 最高价(%.3f) < 最低价(%.3f)\n", q.K.High.Float64(), q.K.Low.Float64())
		}
		if q.K.High.Float64() < q.K.Close.Float64() {
			fmt.Printf("  [错误] 最高价(%.3f) < 收盘价(%.3f)\n", q.K.High.Float64(), q.K.Close.Float64())
		}
		if q.K.Low.Float64() > q.K.Close.Float64() {
			fmt.Printf("  [错误] 最低价(%.3f) > 收盘价(%.3f)\n", q.K.Low.Float64(), q.K.Close.Float64())
		}

		// 验证五档买卖价递增/递减
		fmt.Printf("  总手: %d  现量: %d  金额: %.0f  内盘: %d  外盘: %d\n",
			q.TotalHand, q.Intuition, q.Amount, q.InsideDish, q.OuterDisc)
		fmt.Printf("  内盘+外盘=%d, 总手=%d, 差值=%d\n",
			q.InsideDish+q.OuterDisc, q.TotalHand, q.TotalHand-(q.InsideDish+q.OuterDisc))

		// 五档买盘(买1>买2>...>买5)
		fmt.Println("  买盘:")
		for i, v := range q.BuyLevel {
			fmt.Printf("    买%d: 价格=%.3f  量=%d\n", i+1, v.Price.Float64(), v.Number)
		}
		// 验证买盘价格递减
		for i := 1; i < 5; i++ {
			if q.BuyLevel[i-1].Price.Float64() < q.BuyLevel[i].Price.Float64() && q.BuyLevel[i].Price.Float64() > 0 {
				fmt.Printf("    [错误] 买%d(%.3f) < 买%d(%.3f), 应递减\n",
					i, q.BuyLevel[i-1].Price.Float64(), i+1, q.BuyLevel[i].Price.Float64())
			}
		}
		// 五档卖盘(卖1<卖2<...<卖5)
		fmt.Println("  卖盘:")
		for i, v := range q.SellLevel {
			fmt.Printf("    卖%d: 价格=%.3f  量=%d\n", i+1, v.Price.Float64(), v.Number)
		}
		// 验证卖盘价格递增
		for i := 1; i < 5; i++ {
			if q.SellLevel[i-1].Price.Float64() > q.SellLevel[i].Price.Float64() && q.SellLevel[i-1].Price.Float64() > 0 {
				fmt.Printf("    [错误] 危%d(%.3f) > 危%d(%.3f), 应递增\n",
					i, q.SellLevel[i-1].Price.Float64(), i+1, q.SellLevel[i].Price.Float64())
			}
		}

		// 验证买1<卖1
		if q.BuyLevel[0].Price.Float64() > 0 && q.SellLevel[0].Price.Float64() > 0 {
			spread := q.SellLevel[0].Price.Float64() - q.BuyLevel[0].Price.Float64()
			fmt.Printf("  买1-卖1价差: %.3f\n", spread)
			if spread < 0 {
				fmt.Printf("  [错误] 买1(%.3f) > 危1(%.3f), 买1应<危1\n",
					q.BuyLevel[0].Price.Float64(), q.SellLevel[0].Price.Float64())
			}
		}

		// 未知字段
		fmt.Printf("  未知字段: Rev0=%d Rev1=%d Rev2=%d Rev3=%d Rev4=%d Rev5=%d Rev6=%d Rev7=%d Rev8=%d Rev9=%d\n",
			q.ReversedBytes0, q.ReversedBytes1, q.ReversedBytes2, q.ReversedBytes3,
			q.ReversedBytes4, q.ReversedBytes5, q.ReversedBytes6, q.ReversedBytes7,
			q.ReversedBytes8, q.ReversedBytes9)
		fmt.Printf("  涨速: %.2f  活跃度1: %d  活跃度2: %d\n", q.Rate, q.Active1, q.Active2)
	}
	fmt.Println()
}

// ==================== 日K线验证 ====================
func verifyKlineDay(c *tdx.Client) {
	fmt.Println("========== 日K线验证 ==========")

	testCodes := []struct {
		code string
		name string
	}{
		{"000001", "平安银行"},
		{"600519", "贵州茅台"},
		{"159558", "ETF"},
	}

	for _, tc := range testCodes {
		fmt.Printf("\n--- %s (%s) 日K线 ---\n", tc.code, tc.name)
		resp, err := c.GetKlineDay(tc.code, 0, 30)
		if err != nil {
			fmt.Printf("  获取失败: %v\n", err)
			continue
		}
		fmt.Printf("  获取到 %d 根K线\n", resp.Count)

		errCount := 0
		for i, k := range resp.List {
			// 验证OHLC关系
			if k.High < k.Low {
				fmt.Printf("  [错误] %s 最高(%.3f)<最低(%.3f)\n", k.Time.Format("2006-01-02"), k.High.Float64(), k.Low.Float64())
				errCount++
			}
			if k.High < k.Open {
				fmt.Printf("  [错误] %s 最高(%.3f)<开盘(%.3f)\n", k.Time.Format("2006-01-02"), k.High.Float64(), k.Open.Float64())
				errCount++
			}
			if k.High < k.Close {
				fmt.Printf("  [错误] %s 最高(%.3f)<收盘(%.3f)\n", k.Time.Format("2006-01-02"), k.High.Float64(), k.Close.Float64())
				errCount++
			}
			if k.Low > k.Open {
				fmt.Printf("  [错误] %s 最低(%.3f)>开盘(%.3f)\n", k.Time.Format("2006-01-02"), k.Low.Float64(), k.Open.Float64())
				errCount++
			}
			if k.Low > k.Close {
				fmt.Printf("  [错误] %s 最低(%.3f)>收盘(%.3f)\n", k.Time.Format("2006-01-02"), k.Low.Float64(), k.Close.Float64())
				errCount++
			}
			// 验证成交量和成交额非负
			if k.Volume < 0 {
				fmt.Printf("  [错误] %s 成交量为负: %d\n", k.Time.Format("2006-01-02"), k.Volume)
				errCount++
			}
			// 验证时间递增
			if i > 0 && k.Time.Before(resp.List[i-1].Time) {
				fmt.Printf("  [错误] %s 时间非递增(前一条: %s)\n", k.Time.Format("2006-01-02"), resp.List[i-1].Time.Format("2006-01-02"))
				errCount++
			}

			// 打印最近5根
			if i >= int(resp.Count)-5 {
				fmt.Printf("  %s: 开%.3f 高%.3f 低%.3f 收%.3f 量%d 额%.0f 昨收%.3f\n",
					k.Time.Format("2006-01-02"), k.Open.Float64(), k.High.Float64(), k.Low.Float64(), k.Close.Float64(),
					k.Volume, k.Amount.Float64(), k.Last.Float64())
			}
		}
		if errCount == 0 {
			fmt.Printf("  OHLC关系验证通过 (%d根K线)\n", resp.Count)
		} else {
			fmt.Printf("  发现 %d 个错误\n", errCount)
		}
	}
	fmt.Println()
}

// ==================== 分钟K线验证 ====================
func verifyKlineMinute(c *tdx.Client) {
	fmt.Println("========== 分钟K线验证 ==========")

	resp, err := c.GetKlineMinute("000001", 0, 50)
	if err != nil {
		fmt.Printf("获取1分钟K线失败: %v\n", err)
		return
	}
	fmt.Printf("获取到 %d 根1分钟K线\n", resp.Count)

	errCount := 0
	for i, k := range resp.List {
		if k.High < k.Low || k.High < k.Open || k.High < k.Close || k.Low > k.Open || k.Low > k.Close {
			errCount++
		}
		if i > 0 && k.Time.Before(resp.List[i-1].Time) {
			fmt.Printf("  [错误] 时间非递增: %s < %s\n", k.Time.Format("15:04"), resp.List[i-1].Time.Format("15:04"))
			errCount++
		}
		if i >= int(resp.Count)-5 {
			fmt.Printf("  %s: 开%.3f 高%.3f 低%.3f 收%.3f 量%d\n",
				k.Time.Format("2006-01-02 15:04"), k.Open.Float64(), k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume)
		}
	}
	if errCount == 0 {
		fmt.Printf("1分钟K线OHLC+时间验证通过 (%d根)\n", resp.Count)
	} else {
		fmt.Printf("发现 %d 个错误\n", errCount)
	}

	// 5分钟K线
	resp5, err := c.GetKline5Minute("000001", 0, 30)
	if err != nil {
		fmt.Printf("获取5分钟K线失败: %v\n", err)
		return
	}
	fmt.Printf("\n获取到 %d 根5分钟K线\n", resp5.Count)
	for i, k := range resp5.List {
		if i >= int(resp5.Count)-3 {
			fmt.Printf("  %s: 开%.3f 高%.3f 低%.3f 收%.3f 量%d\n",
				k.Time.Format("2006-01-02 15:04"), k.Open.Float64(), k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume)
		}
	}
	fmt.Println()
}

// ==================== 分时成交验证 ====================
func verifyTrade(c *tdx.Client) {
	fmt.Println("========== 分时成交验证 ==========")

	resp, err := c.GetMinuteTrade("000001", 0, 200)
	if err != nil {
		fmt.Printf("获取分时成交失败: %v\n", err)
		return
	}
	fmt.Printf("获取到 %d 条分时成交\n", resp.Count)

	buyCount := 0
	sellCount := 0
	neutralCount := 0
	var totalBuyVol, totalSellVol int64

	for i, t := range resp.List {
		switch t.Status {
		case 0:
			buyCount++
			totalBuyVol += int64(t.Volume) * 100
		case 1:
			sellCount++
			totalSellVol += int64(t.Volume) * 100
		default:
			neutralCount++
		}

		if i < 5 || i >= int(resp.Count)-5 {
			fmt.Printf("  %s: 价格%.3f 量%d手 状态%d(%s) 单数%d\n",
				t.Time.Format("15:04:05"), t.Price.Float64(), t.Volume, t.Status, t.StatusString(), t.Number)
		}
	}

	fmt.Printf("\n  买入: %d条(量%d股)  卖出: %d条(量%d股)  中性: %d条\n",
		buyCount, totalBuyVol, sellCount, totalSellVol, neutralCount)
	fmt.Printf("  内盘/外盘比: 买%.1f%% 危%.1f%%\n",
		float64(totalBuyVol)/float64(totalBuyVol+totalSellVol)*100,
		float64(totalSellVol)/float64(totalBuyVol+totalSellVol)*100)

	// 历史分时成交
	fmt.Println("\n--- 历史分时成交 (最近交易日) ---")
	date := time.Now().Add(-1 * time.Hour * 24).Format("20060102")
	resp2, err := c.GetHistoryMinuteTrade(date, "000001", 0, 200)
	if err != nil {
		fmt.Printf("  获取历史分时成交失败: %v\n", err)
	} else {
		fmt.Printf("  日期=%s 获取到 %d 条\n", date, resp2.Count)
		for i, t := range resp2.List {
			if i < 3 {
				fmt.Printf("  %s: 价格%.3f 量%d手 状态%d\n",
					t.Time.Format("15:04"), t.Price.Float64(), t.Volume, t.Status)
			}
		}
	}
	fmt.Println()
}

// ==================== 财务信息验证 ====================
func verifyFinanceInfo(c *tdx.Client) {
	fmt.Println("========== 财务信息验证 ==========")

	testCases := []struct {
		exchange protocol.Exchange
		code     string
		name     string
	}{
		{protocol.ExchangeSH, "600519", "贵州茅台"},
		{protocol.ExchangeSZ, "000001", "平安银行"},
	}

	for _, tc := range testCases {
		fmt.Printf("\n--- %s (%s) ---\n", tc.code, tc.name)
		fi, err := c.GetFinanceInfo(tc.exchange, tc.code)
		if err != nil {
			fmt.Printf("  获取失败: %v\n", err)
			continue
		}

		fmt.Printf("  流通股本: %.0f  总股本: %.0f\n", fi.LiuTongGuBen, fi.ZongGuBen)
		fmt.Printf("  行业码: %d  地域码: %d\n", fi.Industry, fi.Province)
		fmt.Printf("  IPO日期: %d  更新日期: %d\n", fi.IPODate, fi.UpdatedDate)
		fmt.Printf("  总资产: %.0f  净资产: %.0f\n", fi.ZongZiChan, fi.JingZiChan)
		fmt.Printf("  主营收入: %.0f  净利润: %.0f\n", fi.ZhuYingShouRu, fi.JingLiRun)
		fmt.Printf("  股东户数: %.0f\n", fi.GuDongRenShu)
		fmt.Printf("  保留1: %.0f  保留2: %.0f\n", fi.BaoLiu1, fi.BaoLiu2)

		// 验证基本合理性
		if fi.ZongGuBen > 0 && fi.LiuTongGuBen > fi.ZongGuBen {
			fmt.Printf("  [警告] 流通股本(%.0f) > 总股本(%.0f)\n", fi.LiuTongGuBen, fi.ZongGuBen)
		}
		if fi.IPODate > 0 {
			ipoYear := fi.IPODate / 10000
			if ipoYear < 1990 || ipoYear > 2026 {
				fmt.Printf("  [警告] IPO日期异常: %d\n", fi.IPODate)
			}
		}
	}
	fmt.Println()
}

// ==================== 股本变迁/复权验证 ====================
func verifyGbbq(c *tdx.Client) {
	fmt.Println("========== 股本变迁/复权验证 ==========")

	testCodes := []string{"600519", "000001", "000651"}
	for _, code := range testCodes {
		fmt.Printf("\n--- %s ---\n", code)
		resp, err := c.GetGbbq(code)
		if err != nil {
			fmt.Printf("  获取失败: %v\n", err)
			continue
		}
		fmt.Printf("  获取到 %d 条股本变迁记录\n", resp.Count)

		xrxdCount := 0
		equityCount := 0
		otherCount := 0
		for i, g := range resp.List {
			switch {
			case g.IsXRXD():
				xrxdCount++
			case g.IsEquity():
				equityCount++
			default:
				otherCount++
			}
			if i < 5 {
				catName := "其他"
				switch g.Category {
				case 1:
					catName = "除权除息"
				case 11, 12:
					catName = "扩缩股"
				case 13, 14:
					catName = "权证"
				default:
					catName = "股本变动"
				}
				fmt.Printf("  %s Cat=%d(%s) C1=%.4f C2=%.4f C3=%.4f C4=%.4f\n",
					g.Time.Format("2006-01-02"), g.Category, catName, g.C1, g.C2, g.C3, g.C4)
			}
		}
		fmt.Printf("  除权除息: %d  股本变动: %d  其他: %d\n", xrxdCount, equityCount, otherCount)
	}
	fmt.Println()
}

// ==================== 指数K线验证 ====================
func verifyIndexKline(c *tdx.Client) {
	fmt.Println("========== 指数K线验证 ==========")

	resp, err := c.GetIndexDay("000001", 0, 10)
	if err != nil {
		fmt.Printf("获取上证指数日K线失败: %v\n", err)
		return
	}
	fmt.Printf("上证指数日K线: %d根\n", resp.Count)
	for i, k := range resp.List {
		if i >= int(resp.Count)-5 {
			fmt.Printf("  %s: 开%.3f 高%.3f 低%.3f 收%.3f 量%d 涨%d/跌%d\n",
				k.Time.Format("2006-01-02"), k.Open.Float64(), k.High.Float64(), k.Low.Float64(), k.Close.Float64(),
				k.Volume, k.UpCount, k.DownCount)
		}
		// 验证涨跌数
		if k.UpCount < 0 || k.DownCount < 0 {
			fmt.Printf("  [错误] %s 涨跌数异常: 涨%d 跌%d\n", k.Time.Format("2006-01-02"), k.UpCount, k.DownCount)
		}
	}
	fmt.Println()
}

// ==================== 扩展行情验证 ====================
func verifyExHq() {
	fmt.Println("========== 扩展行情验证 ==========")

	fmt.Println("连接扩展行情服务器...")
	ex, err := tdx.DialExHqDefault()
	if err != nil {
		fmt.Printf("连接扩展行情服务器失败: %v\n", err)
		fmt.Println("跳过扩展行情验证")
		return
	}
	defer ex.Close()
	fmt.Println("连接成功")

	// 市场列表
	markets, err := ex.ExMarkets()
	if err != nil {
		fmt.Printf("获取市场列表失败: %v\n", err)
		return
	}
	fmt.Printf("\n市场列表: %d个\n", len(markets))
	for i, m := range markets {
		if i < 10 {
			b, _ := json.Marshal(m)
			fmt.Printf("  %s\n", string(b))
		}
	}
	if len(markets) > 10 {
		fmt.Printf("  ... 共%d个\n", len(markets))
	}

	// 品种数量
	count, err := ex.ExCount()
	if err != nil {
		fmt.Printf("获取品种数量失败: %v\n", err)
	} else {
		fmt.Printf("\n品种总数: %d\n", count)
	}

	// 港股五档(腾讯 00700)
	fmt.Println("\n--- 港股腾讯(00700)五档 ---")
	quote, err := ex.ExQuote(31, "00700")
	if err != nil {
		fmt.Printf("获取港股五档失败: %v\n", err)
	} else if quote != nil {
		b, _ := json.Marshal(quote)
		fmt.Printf("  %s\n", string(b))
		// 验证OHLC
		if quote.High < quote.Low {
			fmt.Println("  [错误] 最高<最低")
		}
	}

	// 期货K线(IF主力)
	fmt.Println("\n--- 中金所IF主力K线 ---")
	bars, err := ex.ExBars(3, 47, "IFM0", 0, 10)
	if err != nil {
		fmt.Printf("获取期货K线失败: %v\n", err)
	} else {
		fmt.Printf("  获取到 %d 根K线\n", len(bars))
		for i, b := range bars {
			if i >= len(bars)-3 {
				fmt.Printf("  %s: 开%.2f 高%.2f 低%.2f 收%.2f 量%d 持仓%d 额%.0f\n",
					b.Datetime, b.Open, b.High, b.Low, b.Close, b.Trade, b.Position, b.Amount)
			}
			if b.High < b.Low {
				fmt.Printf("  [错误] %s 最高<最低\n", b.Datetime)
			}
		}
	}

	// 港股分笔成交
	fmt.Println("\n--- 港股腾讯(00700)分笔成交 ---")
	trades, err := ex.ExTrade(31, "00700", 0, 20)
	if err != nil {
		fmt.Printf("获取港股分笔成交失败: %v\n", err)
	} else {
		fmt.Printf("  获取到 %d 条\n", len(trades))
		for i, t := range trades {
			if i < 5 {
				fmt.Printf("  %02d:%02d:%02d 价格=%d 量=%d 增仓=%d 方向=%d(%s)\n",
					t.Hour, t.Minute, t.Second, t.Price, t.Volume, t.ZengCang, t.Direction, t.NatureName)
			}
		}
	}

	fmt.Println()
}

// ==================== 辅助函数 ====================
func approxEqual(a, b float64, eps float64) bool {
	return math.Abs(a-b) < eps
}

func mustJson(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// 忽略未使用变量警告
var _ = approxEqual
var _ = mustJson
var _ = strings.NewReader
var _ = os.Getenv
