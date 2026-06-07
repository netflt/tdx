package main

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

func main() {
	fmt.Println("=== 分时成交数据完整性验证 ===")
	fmt.Println()

	c, err := tdx.DialWith(tdx.NewHostDial(tdx.Hosts))
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer c.Close()
	tdx.DefaultCodes, err = tdx.NewCodesSqlite(tdx.WithCodesClient(c))
	if err != nil {
		fmt.Printf("代码表初始化失败: %v\n", err)
		return
	}

	// ========== 1. 当日分时成交 ==========
	fmt.Println("========================================")
	fmt.Println("一、当日分时成交字段完整性检查")
	fmt.Println("========================================")

	testCodes := []struct {
		code string
		name string
	}{
		{"000001", "平安银行"},
		{"600519", "贵州茅台"},
		{"000651", "格力电器"},
		{"601318", "中国平安"},
		{"159558", "ETF"},
	}

	for _, tc := range testCodes {
		fmt.Printf("\n--- %s (%s) ---\n", tc.code, tc.name)

		// 获取当日分时成交(前1800条)
		resp, err := c.GetMinuteTrade(tc.code, 0, 1800)
		if err != nil {
			fmt.Printf("  获取失败: %v\n", err)
			continue
		}
		fmt.Printf("  获取到 %d 条分时成交\n", resp.Count)

		if resp.Count == 0 {
			fmt.Println("  [注意] 无数据(可能非交易时段)")
			continue
		}

		// --- 字段完整性检查 ---
		checkTradeFields(resp, tc.code)
	}

	// ========== 2. 历史分时成交 ==========
	fmt.Println("\n========================================")
	fmt.Println("二、历史分时成交完整性检查")
	fmt.Println("========================================")

	// 取最近几个交易日
	dates := findRecentTradeDates(c, 3)
	for _, date := range dates {
		fmt.Printf("\n--- 日期: %s ---\n", date)
		resp, err := c.GetHistoryMinuteTradeDay(date, "000001")
		if err != nil {
			fmt.Printf("  获取失败: %v\n", err)
			continue
		}
		fmt.Printf("  平安银行: %d 条\n", resp.Count)
		if resp.Count > 0 {
			checkTradeFields(resp, "000001")
		}

		// 茅台
		resp2, err := c.GetHistoryMinuteTradeDay(date, "600519")
		if err != nil {
			fmt.Printf("  茅台获取失败: %v\n", err)
			continue
		}
		fmt.Printf("  贵州茅台: %d 条\n", resp2.Count)
		if resp2.Count > 0 {
			checkTradeFields(resp2, "600519")
		}
	}

	// ========== 3. 分时成交 vs K线一致性 ==========
	fmt.Println("\n========================================")
	fmt.Println("三、分时成交与日K线一致性验证")
	fmt.Println("========================================")

	for _, date := range dates {
		fmt.Printf("\n--- 日期: %s ---\n", date)

		for _, tc := range []struct {
			code string
			name string
		}{
			{"000001", "平安银行"},
			{"600519", "贵州茅台"},
		} {
			// 分时成交
			tradeResp, err := c.GetHistoryMinuteTradeDay(date, tc.code)
			if err != nil || tradeResp.Count == 0 {
				continue
			}

			// 日K线
			klineResp, err := c.GetKlineDay(tc.code, 0, 30)
			if err != nil {
				continue
			}

			// 找到对应日期的K线
			var targetKline *protocol.Kline
			for _, k := range klineResp.List {
				if k.Time.Format("20060102") == date {
					targetKline = k
					break
				}
			}
			if targetKline == nil {
				continue
			}

			// 从分时成交汇总
			var totalVolume int64
			var totalAmount float64
			var openPrice, closePrice, highPrice, lowPrice protocol.Price
			firstTrade := true
			for _, t := range tradeResp.List {
				totalVolume += int64(t.Volume) * 100 // 手->股
				totalAmount += t.Price.Float64() * float64(t.Volume) * 100

				if firstTrade {
					openPrice = t.Price
					closePrice = t.Price
					highPrice = t.Price
					lowPrice = t.Price
					firstTrade = false
				} else {
					closePrice = t.Price
					if t.Price > highPrice {
						highPrice = t.Price
					}
					if t.Price < lowPrice {
						lowPrice = t.Price
					}
				}
			}

			fmt.Printf("  %s(%s):\n", tc.code, tc.name)
			fmt.Printf("    开盘: 分时=%.3f K线=%.3f 差=%.3f\n",
				openPrice.Float64(), targetKline.Open.Float64(), openPrice.Float64()-targetKline.Open.Float64())
			fmt.Printf("    最高: 分时=%.3f K线=%.3f 差=%.3f\n",
				highPrice.Float64(), targetKline.High.Float64(), highPrice.Float64()-targetKline.High.Float64())
			fmt.Printf("    最低: 分时=%.3f K线=%.3f 差=%.3f\n",
				lowPrice.Float64(), targetKline.Low.Float64(), lowPrice.Float64()-targetKline.Low.Float64())
			fmt.Printf("    收盘: 分时=%.3f K线=%.3f 差=%.3f\n",
				closePrice.Float64(), targetKline.Close.Float64(), closePrice.Float64()-targetKline.Close.Float64())
			fmt.Printf("    成交量: 分时=%d K线=%d 差=%d (%.2f%%)\n",
				totalVolume, targetKline.Volume, totalVolume-targetKline.Volume,
				float64(totalVolume-targetKline.Volume)/float64(targetKline.Volume)*100)
			fmt.Printf("    成交额: 分时=%.0f K线=%.0f 差=%.0f (%.2f%%)\n",
				totalAmount, targetKline.Amount.Float64(), totalAmount-targetKline.Amount.Float64(),
				(totalAmount-targetKline.Amount.Float64())/targetKline.Amount.Float64()*100)
		}
	}

	// ========== 4. 分时成交 vs 1分钟K线一致性 ==========
	fmt.Println("\n========================================")
	fmt.Println("四、分时成交合并为1分钟K线 vs 服务器1分钟K线")
	fmt.Println("========================================")

	for _, date := range dates {
		fmt.Printf("\n--- 日期: %s ---\n", date)

		tradeResp, err := c.GetHistoryMinuteTradeDay(date, "000001")
		if err != nil || tradeResp.Count == 0 {
			fmt.Println("  分时成交无数据")
			continue
		}

		// 合并为1分钟K线
		tradeKlines := protocol.Trades(tradeResp.List).Klines()

		// 获取服务器1分钟K线
		minKlines, err := c.GetKlineMinute("000001", 0, 800)
		if err != nil {
			fmt.Printf("  1分钟K线获取失败: %v\n", err)
			continue
		}

		// 按日期过滤
		var serverKlines protocol.Klines
		for _, k := range minKlines.List {
			if k.Time.Format("20060102") == date {
				serverKlines = append(serverKlines, k)
			}
		}

		fmt.Printf("  分时成交合并: %d根1分钟K线\n", len(tradeKlines))
		fmt.Printf("  服务器1分钟K线: %d根\n", len(serverKlines))

		// 按时间匹配比对
		serverMap := make(map[string]*protocol.Kline)
		for _, k := range serverKlines {
			serverMap[k.Time.Format("15:04")] = k
		}

		matchCount := 0
		priceMatch := 0
		volMatch := 0
		for _, tk := range tradeKlines {
			key := tk.Time.Format("15:04")
			sk, ok := serverMap[key]
			if !ok {
				continue
			}
			matchCount++

			closeDiff := math.Abs(tk.Close.Float64() - sk.Close.Float64())
			if closeDiff < 0.005 {
				priceMatch++
			} else {
				fmt.Printf("    [价格差异] %s: 分时合并=%.3f 服务器=%.3f 差=%.3f\n",
					key, tk.Close.Float64(), sk.Close.Float64(), closeDiff)
			}

			volDiff := tk.Volume - sk.Volume
			if volDiff == 0 {
				volMatch++
			} else {
				fmt.Printf("    [量差异] %s: 分时合并=%d 服务器=%d 差=%d\n",
					key, tk.Volume, sk.Volume, volDiff)
			}
		}

		if matchCount > 0 {
			fmt.Printf("  匹配 %d 根, 价格一致 %d 根(%.1f%%), 成交量一致 %d 根(%.1f%%)\n",
				matchCount, priceMatch, float64(priceMatch)/float64(matchCount)*100,
				volMatch, float64(volMatch)/float64(matchCount)*100)
		}
	}

	// ========== 5. 边界情况 ==========
	fmt.Println("\n========================================")
	fmt.Println("五、边界情况检查")
	fmt.Println("========================================")

	// 5.1 分页连续性
	fmt.Println("\n--- 分页连续性(当日) ---")
	resp1, err1 := c.GetMinuteTrade("000001", 0, 500)
	resp2, err2 := c.GetMinuteTrade("000001", 500, 500)
	if err1 == nil && err2 == nil {
		fmt.Printf("  第1页: %d条, 第2页: %d条\n", resp1.Count, resp2.Count)
		if resp1.Count > 0 && resp2.Count > 0 {
			lastOfPage1 := resp1.List[len(resp1.List)-1]
			firstOfPage2 := resp2.List[0]
			fmt.Printf("  第1页最后: %s 价格=%.3f 量=%d\n",
				lastOfPage1.Time.Format("15:04"), lastOfPage1.Price.Float64(), lastOfPage1.Volume)
			fmt.Printf("  第2页第1条: %s 价格=%.3f 量=%d\n",
				firstOfPage2.Time.Format("15:04"), firstOfPage2.Price.Float64(), firstOfPage2.Volume)
			// 时间应递增
			if firstOfPage2.Time.After(lastOfPage1.Time) || firstOfPage2.Time.Equal(lastOfPage1.Time) {
				fmt.Println("  分页时间连续: OK")
			} else {
				fmt.Println("  [错误] 分页时间不连续!")
			}
		}
	}

	// 5.2 历史分页连续性
	fmt.Println("\n--- 历史分时成交分页连续性 ---")
	if len(dates) > 0 {
		date := dates[0]
		hresp1, err1 := c.GetHistoryMinuteTrade(date, "000001", 0, 1000)
		hresp2, err2 := c.GetHistoryMinuteTrade(date, "000001", 1000, 1000)
		if err1 == nil && err2 == nil {
			fmt.Printf("  日期=%s 第1页: %d条, 第2页: %d条\n", date, hresp1.Count, hresp2.Count)
			if hresp1.Count > 0 && hresp2.Count > 0 {
				last := hresp1.List[len(hresp1.List)-1]
				first := hresp2.List[0]
				fmt.Printf("  第1页最后: %s 价格=%.3f\n", last.Time.Format("15:04"), last.Price.Float64())
				fmt.Printf("  第2页第1条: %s 价格=%.3f\n", first.Time.Format("15:04"), first.Price.Float64())
				if first.Time.After(last.Time) || first.Time.Equal(last.Time) {
					fmt.Println("  历史分页时间连续: OK")
				} else {
					fmt.Println("  [错误] 历史分页时间不连续!")
				}
			}
		}
	}

	// 5.3 非交易日
	fmt.Println("\n--- 非交易日请求 ---")
	nonTradeDate := "20260101" // 元旦
	ntr, err := c.GetHistoryMinuteTrade(nonTradeDate, "000001", 0, 100)
	if err != nil {
		fmt.Printf("  非交易日(%s)返回错误: %v\n", nonTradeDate, err)
	} else {
		fmt.Printf("  非交易日(%s)返回 %d 条数据\n", nonTradeDate, ntr.Count)
	}

	// 5.4 不存在的股票
	fmt.Println("\n--- 不存在的股票 ---")
	badResp, err := c.GetMinuteTrade("999999", 0, 100)
	if err != nil {
		fmt.Printf("  不存在股票返回错误: %v\n", err)
	} else {
		fmt.Printf("  不存在股票返回 %d 条数据\n", badResp.Count)
	}

	// 5.5 ETF分时成交
	fmt.Println("\n--- ETF分时成交 ---")
	etfResp, err := c.GetMinuteTrade("159558", 0, 100)
	if err != nil {
		fmt.Printf("  ETF获取失败: %v\n", err)
	} else {
		fmt.Printf("  ETF(159558)获取到 %d 条\n", etfResp.Count)
		if etfResp.Count > 0 {
			for i, t := range etfResp.List {
				if i < 3 {
					fmt.Printf("    %s: 价格=%.3f 量=%d手 状态=%d\n",
						t.Time.Format("15:04"), t.Price.Float64(), t.Volume, t.Status)
				}
			}
		}
	}

	// 5.6 北交所股票
	fmt.Println("\n--- 北交所股票分时成交 ---")
	bjResp, err := c.GetMinuteTrade("920010", 0, 100)
	if err != nil {
		fmt.Printf("  北交所获取失败: %v\n", err)
	} else {
		fmt.Printf("  北交所(920010)获取到 %d 条\n", bjResp.Count)
		if bjResp.Count > 0 {
			for i, t := range bjResp.List {
				if i < 3 {
					fmt.Printf("    %s: 价格=%.3f 量=%d手 状态=%d\n",
						t.Time.Format("15:04"), t.Price.Float64(), t.Volume, t.Status)
				}
			}
		}
	}

	fmt.Println("\n=== 验证完成 ===")
}

func checkTradeFields(resp *protocol.TradeResp, code string) {
	total := len(resp.List)
	if total == 0 {
		return
	}

	// 统计各字段
	buyCount, sellCount, neutralCount := 0, 0, 0
	zeroVolCount := 0
	zeroPriceCount := 0
	zeroNumberCount := 0
	negativeVolCount := 0
	timeNoSecondCount := 0
	var minPrice, maxPrice protocol.Price
	var totalVol int64
	var totalAmount float64
	statusValues := make(map[int]int)

	minPrice = resp.List[0].Price
	maxPrice = resp.List[0].Price

	for i, t := range resp.List {
		// Status统计
		switch t.Status {
		case 0:
			buyCount++
		case 1:
			sellCount++
		default:
			neutralCount++
		}
		statusValues[t.Status]++

		// 价格检查
		if t.Price == 0 {
			zeroPriceCount++
		}
		if t.Price < minPrice {
			minPrice = t.Price
		}
		if t.Price > maxPrice {
			maxPrice = t.Price
		}

		// 成交量检查
		if t.Volume == 0 {
			zeroVolCount++
		}
		if t.Volume < 0 {
			negativeVolCount++
		}
		totalVol += int64(t.Volume)

		// 单数检查
		if t.Number == 0 {
			zeroNumberCount++
		}

		// 成交额
		totalAmount += t.Price.Float64() * float64(t.Volume) * 100

		// 时间精度(秒是否为0)
		if t.Time.Second() == 0 {
			timeNoSecondCount++
		}

		// 时间递增检查
		if i > 0 && t.Time.Before(resp.List[i-1].Time) {
			fmt.Printf("  [错误] 第%d条时间非递增: %s < %s\n", i, t.Time.Format("15:04:05"), resp.List[i-1].Time.Format("15:04:05"))
		}
	}

	fmt.Printf("  字段统计:\n")
	fmt.Printf("    买卖方向: 买=%d(%.1f%%) 卖=%d(%.1f%%) 中性=%d(%.1f%%)\n",
		buyCount, float64(buyCount)/float64(total)*100,
		sellCount, float64(sellCount)/float64(total)*100,
		neutralCount, float64(neutralCount)/float64(total)*100)
	fmt.Printf("    Status值分布: %v\n", statusValues)
	fmt.Printf("    价格范围: %.3f ~ %.3f\n", minPrice.Float64(), maxPrice.Float64())
	fmt.Printf("    零价格: %d条\n", zeroPriceCount)
	fmt.Printf("    零成交量: %d条\n", zeroVolCount)
	fmt.Printf("    负成交量: %d条\n", negativeVolCount)
	fmt.Printf("    零单数: %d条(%.1f%%)\n", zeroNumberCount, float64(zeroNumberCount)/float64(total)*100)
	fmt.Printf("    时间无秒精度: %d条(%.1f%%)\n", timeNoSecondCount, float64(timeNoSecondCount)/float64(total)*100)
	fmt.Printf("    总成交量: %d手(%d股)\n", totalVol, totalVol*100)
	fmt.Printf("    总成交额: %.0f元\n", totalAmount)

	// 内盘外盘 vs 分时成交买卖量
	sellVol, buyVol := protocol.Trades(resp.List).Volume2()
	fmt.Printf("    内盘(主动卖): %d股  外盘(主动买): %d股\n", sellVol, buyVol)
	fmt.Printf("    内盘占比: %.2f%%  外盘占比: %.2f%%\n",
		float64(sellVol)/float64(sellVol+buyVol)*100,
		float64(buyVol)/float64(sellVol+buyVol)*100)

	// 时间覆盖检查
	timeSet := make(map[string]bool)
	for _, t := range resp.List {
		timeSet[t.Time.Format("15:04")] = true
	}
	fmt.Printf("    时间覆盖: %d个不同分钟\n", len(timeSet))

	// 首尾时间
	fmt.Printf("    首条时间: %s  末条时间: %s\n",
		resp.List[0].Time.Format("15:04:05"), resp.List[total-1].Time.Format("15:04:05"))
}

func findRecentTradeDates(c *tdx.Client, n int) []string {
	resp, err := c.GetKlineDay("000001", 0, 30)
	if err != nil {
		return nil
	}
	dates := make([]string, 0, n)
	for i := len(resp.List) - 1; i >= 0 && len(dates) < n; i-- {
		d := resp.List[i].Time.Format("20060102")
		// 跳过今天(可能盘中数据不完整)
		if d == time.Now().Format("20060102") {
			continue
		}
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates
}
