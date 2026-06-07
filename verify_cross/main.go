package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

func main() {
	fmt.Println("=== TDX vs 新浪财经 交叉验证 ===")
	fmt.Println()

	// 连接TDX
	c, err := tdx.DialWith(tdx.NewHostDial(tdx.Hosts))
	if err != nil {
		fmt.Printf("TDX连接失败: %v\n", err)
		return
	}
	defer c.Close()

	tdx.DefaultCodes, err = tdx.NewCodesSqlite(tdx.WithCodesClient(c))
	if err != nil {
		fmt.Printf("代码表初始化失败: %v\n", err)
		return
	}

	// === 1. 五档行情交叉验证 ===
	fmt.Println("========== 五档行情 vs 新浪财经 ==========")
	testCodes := []struct {
		tdxCode  string
		sinaCode string
		name     string
	}{
		{"000001", "sz000001", "平安银行"},
		{"600519", "sh600519", "贵州茅台"},
		{"000651", "sz000651", "格力电器"},
		{"159558", "sz159558", "ETF"},
		{"601318", "sh601318", "中国平安"},
	}

	for _, tc := range testCodes {
		fmt.Printf("\n--- %s (%s) ---\n", tc.tdxCode, tc.name)

		// TDX数据
		tdxQuotes, err := c.GetQuote(tc.tdxCode)
		if err != nil {
			fmt.Printf("  TDX获取失败: %v\n", err)
			continue
		}
		if len(tdxQuotes) == 0 {
			fmt.Println("  TDX返回空数据")
			continue
		}
		q := tdxQuotes[0]

		// 新浪数据
		sinaData := fetchSinaQuote(tc.sinaCode)
		if sinaData == nil {
			fmt.Println("  新浪数据获取失败")
			continue
		}

		// 交叉比对
		fmt.Printf("  %-12s  %-12s  %-12s  %-12s\n", "字段", "TDX", "新浪", "差值")
		fmt.Printf("  %-12s  %-12s  %-12s  %-12s\n", "----", "---", "----", "----")

		comparePrice("昨收", q.K.Last.Float64(), sinaData.LastClose)
		comparePrice("今开", q.K.Open.Float64(), sinaData.Open)
		comparePrice("最高", q.K.High.Float64(), sinaData.High)
		comparePrice("最低", q.K.Low.Float64(), sinaData.Low)
		comparePrice("最新", q.K.Close.Float64(), sinaData.Price)

		compareInt("总手", q.TotalHand, sinaData.Volume/100)
		compareFloat("金额(万)", q.Amount/10000, sinaData.Amount/10000, 1)

		// 内盘外盘对比
		fmt.Printf("  %-12s  %-12d  %-12d  %-12s\n", "内盘", q.InsideDish, sinaData.InnerVol, "")
		fmt.Printf("  %-12s  %-12d  %-12d  %-12s\n", "外盘", q.OuterDisc, sinaData.OuterVol, "")
		innerDiff := q.InsideDish - sinaData.InnerVol
		outerDiff := q.OuterDisc - sinaData.OuterVol
		fmt.Printf("  内盘差=%d  外盘差=%d  内盘+外盘(TDX)=%d  内盘+外盘(新浪)=%d\n",
			innerDiff, outerDiff, q.InsideDish+q.OuterDisc, sinaData.InnerVol+sinaData.OuterVol)

		// 五档对比
		fmt.Println("  五档买盘对比:")
		for i := 0; i < 5; i++ {
			tdxBidPrice := q.BuyLevel[i].Price.Float64()
			tdxBidVol := q.BuyLevel[i].Number
			sinaBidPrice := sinaData.BidPrices[i]
			sinaBidVol := sinaData.BidVols[i]
			priceDiff := tdxBidPrice - sinaBidPrice
			volDiff := tdxBidVol - sinaBidVol
			fmt.Printf("    买%d: TDX(%.3f/%d) 新浪(%.3f/%d) 价差%.3f 量差%d\n",
				i+1, tdxBidPrice, tdxBidVol, sinaBidPrice, sinaBidVol, priceDiff, volDiff)
		}
		fmt.Println("  五档卖盘对比:")
		for i := 0; i < 5; i++ {
			tdxAskPrice := q.SellLevel[i].Price.Float64()
			tdxAskVol := q.SellLevel[i].Number
			sinaAskPrice := sinaData.AskPrices[i]
			sinaAskVol := sinaData.AskVols[i]
			priceDiff := tdxAskPrice - sinaAskPrice
			volDiff := tdxAskVol - sinaAskVol
			fmt.Printf("    危%d: TDX(%.3f/%d) 新浪(%.3f/%d) 价差%.3f 量差%d\n",
				i+1, tdxAskPrice, tdxAskVol, sinaAskPrice, sinaAskVol, priceDiff, volDiff)
		}
	}

	// === 2. 日K线交叉验证 ===
	fmt.Println("\n========== 日K线 vs 新浪财经 ==========")
	klineCodes := []struct {
		tdxCode  string
		sinaCode string
		name     string
	}{
		{"000001", "sz000001", "平安银行"},
		{"600519", "sh600519", "贵州茅台"},
	}

	for _, tc := range klineCodes {
		fmt.Printf("\n--- %s (%s) 最近5日K线 ---\n", tc.tdxCode, tc.name)

		// TDX日K线
		tdxKlines, err := c.GetKlineDay(tc.tdxCode, 0, 10)
		if err != nil {
			fmt.Printf("  TDX获取失败: %v\n", err)
			continue
		}

		// 新浪日K线
		sinaKlines := fetchSinaKline(tc.sinaCode, 10)
		if len(sinaKlines) == 0 {
			fmt.Println("  新浪K线获取失败")
			continue
		}

		fmt.Printf("  %-12s  %-12s  %-12s  %-12s  %-12s  %-12s\n", "日期", "字段", "TDX", "新浪", "差值", "状态")
		fmt.Printf("  %-12s  %-12s  %-12s  %-12s  %-12s  %-12s\n", "----", "----", "---", "----", "----", "----")

		// 按日期匹配比对
		tdxMap := make(map[string]*protocol.Kline)
		for _, k := range tdxKlines.List {
			key := k.Time.Format("2006-01-02")
			tdxMap[key] = k
		}

		matched := 0
		for _, sk := range sinaKlines {
			tk, ok := tdxMap[sk.Date]
			if !ok {
				continue
			}
			matched++
			if matched > 5 {
				break
			}
			compareKlineField(sk.Date, "开盘", tk.Open.Float64(), sk.Open)
			compareKlineField(sk.Date, "最高", tk.High.Float64(), sk.High)
			compareKlineField(sk.Date, "最低", tk.Low.Float64(), sk.Low)
			compareKlineField(sk.Date, "收盘", tk.Close.Float64(), sk.Close)
		}
		fmt.Printf("  匹配 %d 天\n", matched)
	}

	// === 3. 指数K线验证 ===
	fmt.Println("\n========== 指数K线 vs 新浪财经 ==========")
	indexCodes := []struct {
		tdxCode  string
		sinaCode string
		name     string
	}{
		{"000001", "sh000001", "上证指数"},
		{"399001", "sz399001", "深证成指"},
	}

	for _, tc := range indexCodes {
		fmt.Printf("\n--- %s (%s) ---\n", tc.tdxCode, tc.name)
		tdxKlines, err := c.GetIndexDay(tc.tdxCode, 0, 10)
		if err != nil {
			fmt.Printf("  TDX获取失败: %v\n", err)
			continue
		}

		sinaKlines := fetchSinaKline(tc.sinaCode, 10)
		if len(sinaKlines) == 0 {
			fmt.Println("  新浪K线获取失败")
			continue
		}

		tdxMap := make(map[string]*protocol.Kline)
		for _, k := range tdxKlines.List {
			tdxMap[k.Time.Format("2006-01-02")] = k
		}

		matched := 0
		for _, sk := range sinaKlines {
			tk, ok := tdxMap[sk.Date]
			if !ok {
				continue
			}
			matched++
			if matched > 3 {
				break
			}
			compareKlineField(sk.Date, "收盘", tk.Close.Float64(), sk.Close)
			compareKlineField(sk.Date, "成交量", float64(tk.Volume), float64(sk.Volume), 0.01)
		}
		fmt.Printf("  匹配 %d 天\n", matched)
	}

	fmt.Println("\n=== 交叉验证完成 ===")
}

// ==================== 新浪财经API ====================

type SinaQuote struct {
	Name       string
	Open       float64
	LastClose  float64
	Price      float64
	High       float64
	Low        float64
	Volume     int     // 成交量(股)
	Amount     float64 // 成交额(元)
	InnerVol   int     // 内盘(手)
	OuterVol   int     // 外盘(手)
	BidPrices  [5]float64
	BidVols    [5]int
	AskPrices  [5]float64
	AskVols    [5]int
	Date       string
	Time       string
}

func fetchSinaQuote(code string) *SinaQuote {
	url := fmt.Sprintf("http://hq.sinajs.cn/list=%s", code)
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	line := string(body)
	if !strings.Contains(line, "=\"") {
		return nil
	}
	parts := strings.Split(line, "=\"")
	if len(parts) < 2 {
		return nil
	}
	data := strings.TrimRight(parts[1], "\";\n")
	fields := strings.Split(data, ",")

	if len(fields) < 32 {
		return nil
	}

	q := &SinaQuote{
		Name:      fields[0],
		Open:      atof(fields[1]),
		LastClose: atof(fields[2]),
		Price:     atof(fields[3]),
		High:      atof(fields[4]),
		Low:       atof(fields[5]),
		Volume:    atoi(fields[8]),
		Amount:    atof(fields[9]),
		Date:      fields[30],
		Time:      fields[31],
	}

	// 内盘外盘 (fields[22]=外盘 fields[23]=内盘, 单位手)
	if len(fields) > 24 {
		q.OuterVol = atoi(fields[22])
		q.InnerVol = atoi(fields[23])
	}

	// 五档买盘 (fields[10-19]: 买1量,买1价,...,买5量,买5价)
	for i := 0; i < 5; i++ {
		q.BidVols[i] = atoi(fields[10+i*2])
		q.BidPrices[i] = atof(fields[10+i*2+1])
	}
	// 五档卖盘 (fields[20-29]: 卖1量,危1价,...,危5量,危5价)
	for i := 0; i < 5; i++ {
		q.AskVols[i] = atoi(fields[20+i*2])
		q.AskPrices[i] = atof(fields[20+i*2+1])
	}

	return q
}

type SinaKline struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int
}

func fetchSinaKline(code string, count int) []SinaKline {
	url := fmt.Sprintf("http://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d", code, count)
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var raw []struct {
		Day   string `json:"day"`
		Open  string `json:"open"`
		High  string `json:"high"`
		Low   string `json:"low"`
		Close string `json:"close"`
		Vol   string `json:"volume"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	out := make([]SinaKline, 0, len(raw))
	for _, r := range raw {
		out = append(out, SinaKline{
			Date:   r.Day,
			Open:   atof(r.Open),
			High:   atof(r.High),
			Low:    atof(r.Low),
			Close:  atof(r.Close),
			Volume: atoi(r.Vol),
		})
	}
	return out
}

// ==================== 比较辅助 ====================

func comparePrice(field string, tdx, sina float64) {
	diff := tdx - sina
	status := "OK"
	if math.Abs(diff) > 0.005 {
		status = "DIFF"
	}
	fmt.Printf("  %-12s  %-12.3f  %-12.3f  %-12.3f  %s\n", field, tdx, sina, diff, status)
}

func compareInt(field string, tdx, sina int) {
	diff := tdx - sina
	status := "OK"
	if diff != 0 {
		status = "DIFF"
	}
	fmt.Printf("  %-12s  %-12d  %-12d  %-12d  %s\n", field, tdx, sina, diff, status)
}

func compareFloat(field string, tdx, sina float64, prec float64) {
	diff := tdx - sina
	status := "OK"
	if math.Abs(diff) > prec {
		status = "DIFF"
	}
	fmt.Printf("  %-12s  %-12.0f  %-12.0f  %-12.0f  %s\n", field, tdx, sina, diff, status)
}

func compareKlineField(date, field string, tdx, sina float64, eps ...float64) {
	tol := 0.005
	if len(eps) > 0 {
		tol = eps[0]
	}
	diff := tdx - sina
	status := "OK"
	if math.Abs(diff) > tol {
		status = "DIFF"
	}
	fmt.Printf("  %-12s  %-12s  %-12.3f  %-12.3f  %-12.3f  %s\n", date, field, tdx, sina, diff, status)
}

func atof(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func atoi(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}
