package main

import (
	"fmt"
	"math"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

func main() {
	fmt.Println("=== 修复后分时成交全量数据验证 ===")
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

	// ========== 1. 当日全量 ==========
	fmt.Println("========================================")
	fmt.Println("一、GetMinuteTradeAll 当日全量验证")
	fmt.Println("========================================")

	for _, tc := range []struct {
		code string
		name string
	}{
		{"000001", "平安银行"},
		{"600519", "贵州茅台"},
	} {
		fmt.Printf("\n--- %s (%s) ---\n", tc.code, tc.name)
		resp, err := c.GetMinuteTradeAll(tc.code)
		if err != nil {
			fmt.Printf("  获取失败: %v\n", err)
			continue
		}
		fmt.Printf("  总条数: %d\n", resp.Count)

		// 时间顺序检查
		ascCount := 0
		descCount := 0
		for i := 1; i < len(resp.List); i++ {
			if resp.List[i].Time.After(resp.List[i-1].Time) || resp.List[i].Time.Equal(resp.List[i-1].Time) {
				ascCount++
			} else {
				descCount++
			}
		}
		fmt.Printf("  时间递增: %d  时间递减: %d\n", ascCount, descCount)
		if descCount == 0 {
			fmt.Println("  [OK] 时间完全正序")
		} else {
			fmt.Printf("  [错误] 仍有 %d 处时间递减!\n", descCount)
		}

		if resp.Count > 0 {
			fmt.Printf("  首条: %s 价格=%.3f\n", resp.List[0].Time.Format("15:04"), resp.List[0].Price.Float64())
			fmt.Printf("  末条: %s 价格=%.3f\n", resp.List[len(resp.List)-1].Time.Format("15:04"), resp.List[len(resp.List)-1].Price.Float64())
		}

		// 内盘外盘统计
		sellVol, buyVol := protocol.Trades(resp.List).Volume2()
		fmt.Printf("  内盘(主动卖): %d  外盘(主动买): %d  总量: %d\n", sellVol, buyVol, sellVol+buyVol)
	}

	// ========== 2. 历史全量 ==========
	fmt.Println("\n========================================")
	fmt.Println("二、GetHistoryMinuteTradeDay 历史全量验证")
	fmt.Println("========================================")

	// 找最近交易日
	klines, _ := c.GetKlineDay("000001", 0, 10)
	dates := []string{}
	for _, k := range klines.List {
		d := k.Time.Format("20060102")
		if d != time.Now().Format("20060102") {
			dates = append(dates, d)
		}
	}
	if len(dates) > 3 {
		dates = dates[len(dates)-3:]
	}

	for _, date := range dates {
		fmt.Printf("\n--- 日期: %s ---\n", date)

		for _, tc := range []struct {
			code string
			name string
		}{
			{"000001", "平安银行"},
			{"600519", "贵州茅台"},
		} {
			resp, err := c.GetHistoryMinuteTradeDay(date, tc.code)
			if err != nil {
				fmt.Printf("  %s: 获取失败 %v\n", tc.name, err)
				continue
			}
			fmt.Printf("  %s: %d条\n", tc.name, resp.Count)

			// 时间顺序检查
			descCount := 0
			for i := 1; i < len(resp.List); i++ {
				if resp.List[i].Time.Before(resp.List[i-1].Time) {
					descCount++
				}
			}
			if descCount == 0 {
				fmt.Printf("    [OK] 时间完全正序\n")
			} else {
				fmt.Printf("    [错误] %d处时间递减\n", descCount)
			}

			if resp.Count > 0 {
				fmt.Printf("    首条: %s  末条: %s\n",
					resp.List[0].Time.Format("15:04"), resp.List[len(resp.List)-1].Time.Format("15:04"))
			}
		}
	}

	// ========== 3. 全量 vs 日K线一致性 ==========
	fmt.Println("\n========================================")
	fmt.Println("三、全量分时成交 vs 日K线一致性")
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
			tradeResp, err := c.GetHistoryMinuteTradeDay(date, tc.code)
			if err != nil || tradeResp.Count == 0 {
				continue
			}

			klineResp, err := c.GetKlineDay(tc.code, 0, 30)
			if err != nil {
				continue
			}

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
			var openPrice, closePrice, highPrice, lowPrice protocol.Price
			firstTrade := true
			for _, t := range tradeResp.List {
				totalVolume += int64(t.Volume)
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
			fmt.Printf("    开盘: 分时=%.3f K线=%.3f 差=%.3f %s\n",
				openPrice.Float64(), targetKline.Open.Float64(), openPrice.Float64()-targetKline.Open.Float64(),
				okLabel(math.Abs(openPrice.Float64()-targetKline.Open.Float64()) < 0.005))
			fmt.Printf("    最高: 分时=%.3f K线=%.3f 差=%.3f %s\n",
				highPrice.Float64(), targetKline.High.Float64(), highPrice.Float64()-targetKline.High.Float64(),
				okLabel(math.Abs(highPrice.Float64()-targetKline.High.Float64()) < 0.005))
			fmt.Printf("    最低: 分时=%.3f K线=%.3f 差=%.3f %s\n",
				lowPrice.Float64(), targetKline.Low.Float64(), lowPrice.Float64()-targetKline.Low.Float64(),
				okLabel(math.Abs(lowPrice.Float64()-targetKline.Low.Float64()) < 0.005))
			fmt.Printf("    收盘: 分时=%.3f K线=%.3f 差=%.3f %s\n",
				closePrice.Float64(), targetKline.Close.Float64(), closePrice.Float64()-targetKline.Close.Float64(),
				okLabel(math.Abs(closePrice.Float64()-targetKline.Close.Float64()) < 0.005))
			fmt.Printf("    成交量(手): 分时=%d K线=%d 差=%d %s\n",
				totalVolume, targetKline.Volume, totalVolume-targetKline.Volume,
				okLabel(totalVolume == targetKline.Volume))
		}
	}

	// ========== 4. 全量分时成交 → 1分钟K线 ==========
	fmt.Println("\n========================================")
	fmt.Println("四、全量分时成交合并1分钟K线 vs 服务器1分钟K线")
	fmt.Println("========================================")

	for _, date := range dates {
		fmt.Printf("\n--- 日期: %s ---\n", date)

		tradeResp, err := c.GetHistoryMinuteTradeDay(date, "000001")
		if err != nil || tradeResp.Count == 0 {
			continue
		}

		tradeKlines := protocol.Trades(tradeResp.List).Klines()

		minKlines, err := c.GetKlineMinute("000001", 0, 800)
		if err != nil {
			continue
		}

		var serverKlines protocol.Klines
		for _, k := range minKlines.List {
			if k.Time.Format("20060102") == date {
				serverKlines = append(serverKlines, k)
			}
		}

		fmt.Printf("  分时合并: %d根  服务器: %d根\n", len(tradeKlines), len(serverKlines))

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
			if math.Abs(tk.Close.Float64()-sk.Close.Float64()) < 0.005 {
				priceMatch++
			}
			if tk.Volume == sk.Volume {
				volMatch++
			}
		}

		if matchCount > 0 {
			fmt.Printf("  匹配 %d 根, 价格一致 %d(%.1f%%), 成交量一致 %d(%.1f%%)\n",
				matchCount, priceMatch, float64(priceMatch)/float64(matchCount)*100,
				volMatch, float64(volMatch)/float64(matchCount)*100)
		}
	}

	fmt.Println("\n=== 验证完成 ===")
}

func okLabel(ok bool) string {
	if ok {
		return "[OK]"
	}
	return "[DIFF]"
}
