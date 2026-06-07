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
	fmt.Println("=== TDX 实盘数据综合验证 ===")
	fmt.Println()

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

	// === 1. 五档行情 vs 腾讯财经 ===
	fmt.Println("========== 五档行情 vs 腾讯财经 ==========")
	testCodes := []struct {
		tdxCode   string
		tencentCode string
		name      string
	}{
		{"000001", "0sz000001", "平安银行"},
		{"600519", "0sh600519", "贵州茅台"},
		{"000651", "0sz000651", "格力电器"},
		{"601318", "0sh601318", "中国平安"},
		{"159558", "0sz159558", "ETF"},
	}

	for _, tc := range testCodes {
		fmt.Printf("\n--- %s (%s) ---\n", tc.tdxCode, tc.name)
		tdxQuotes, err := c.GetQuote(tc.tdxCode)
		if err != nil {
			fmt.Printf("  TDX获取失败: %v\n", err)
			continue
		}
		if len(tdxQuotes) == 0 {
			continue
		}
		q := tdxQuotes[0]

		tq := fetchTencentQuote(tc.tencentCode)
		if tq == nil {
			fmt.Println("  腾讯数据获取失败")
			continue
		}

		fmt.Printf("  %-10s  %14s  %14s  %14s  %s\n", "字段", "TDX", "腾讯", "差值", "状态")
		fmt.Printf("  %-10s  %14s  %14s  %14s  %s\n", "----", "---", "----", "----", "----")
		cmpPrice("昨收", q.K.Last.Float64(), tq.LastClose)
		cmpPrice("今开", q.K.Open.Float64(), tq.Open)
		cmpPrice("最高", q.K.High.Float64(), tq.High)
		cmpPrice("最低", q.K.Low.Float64(), tq.Low)
		cmpPrice("最新", q.K.Close.Float64(), tq.Price)

		// 内盘外盘
		fmt.Printf("  %-10s  %14d  %14d  %14d\n", "内盘(手)", q.InsideDish, tq.InnerVol, q.InsideDish-tq.InnerVol)
		fmt.Printf("  %-10s  %14d  %14d  %14d\n", "外盘(手)", q.OuterDisc, tq.OuterVol, q.OuterDisc-tq.OuterVol)
		fmt.Printf("  内盘+外盘: TDX=%d 腾讯=%d 总手TDX=%d 总手腾讯=%d\n",
			q.InsideDish+q.OuterDisc, tq.InnerVol+tq.OuterVol, q.TotalHand, tq.Volume/100)

		// 五档
		fmt.Println("  五档买盘:")
		for i := 0; i < 5; i++ {
			tdxP := q.BuyLevel[i].Price.Float64()
			tdxV := q.BuyLevel[i].Number
			tP := tq.BidPrices[i]
			tV := tq.BidVols[i]
			pDiff := tdxP - tP
			vDiff := tdxV - tV
			status := "OK"
			if math.Abs(pDiff) > 0.005 || vDiff != 0 {
				status = "DIFF"
			}
			fmt.Printf("    买%d: TDX(%.3f/%d) 腾讯(%.3f/%d) 价差%.3f 量差%d %s\n",
				i+1, tdxP, tdxV, tP, tV, pDiff, vDiff, status)
		}
		fmt.Println("  五档卖盘:")
		for i := 0; i < 5; i++ {
			tdxP := q.SellLevel[i].Price.Float64()
			tdxV := q.SellLevel[i].Number
			tP := tq.AskPrices[i]
			tV := tq.AskVols[i]
			pDiff := tdxP - tP
			vDiff := tdxV - tV
			status := "OK"
			if math.Abs(pDiff) > 0.005 || vDiff != 0 {
				status = "DIFF"
			}
			fmt.Printf("    危%d: TDX(%.3f/%d) 腾讯(%.3f/%d) 价差%.3f 量差%d %s\n",
				i+1, tdxP, tdxV, tP, tV, pDiff, vDiff, status)
		}
	}

	// === 2. 日K线 vs 新浪K线 ===
	fmt.Println("\n========== 日K线 vs 新浪财经 ==========")
	klineCodes := []struct {
		tdxCode  string
		sinaCode string
		name     string
	}{
		{"000001", "sz000001", "平安银行"},
		{"600519", "sh600519", "贵州茅台"},
		{"000651", "sz000651", "格力电器"},
		{"601318", "sh601318", "中国平安"},
	}

	for _, tc := range klineCodes {
		fmt.Printf("\n--- %s (%s) ---\n", tc.tdxCode, tc.name)
		tdxKlines, err := c.GetKlineDay(tc.tdxCode, 0, 20)
		if err != nil {
			fmt.Printf("  TDX获取失败: %v\n", err)
			continue
		}
		sinaKlines := fetchSinaKline(tc.sinaCode, 20)
		if len(sinaKlines) == 0 {
			fmt.Println("  新浪K线获取失败")
			continue
		}

		tdxMap := make(map[string]*protocol.Kline)
		for _, k := range tdxKlines.List {
			tdxMap[k.Time.Format("2006-01-02")] = k
		}

		matchCount := 0
		errCount := 0
		for _, sk := range sinaKlines {
			tk, ok := tdxMap[sk.Date]
			if !ok {
				continue
			}
			matchCount++
			if matchCount > 10 {
				break
			}
			fields := []struct{ name string; tdx, sina float64 }{
				{"开盘", tk.Open.Float64(), sk.Open},
				{"最高", tk.High.Float64(), sk.High},
				{"最低", tk.Low.Float64(), sk.Low},
				{"收盘", tk.Close.Float64(), sk.Close},
			}
			for _, f := range fields {
				diff := f.tdx - f.sina
				if math.Abs(diff) > 0.005 {
					errCount++
					fmt.Printf("  [DIFF] %s %s: TDX=%.3f 新浪=%.3f 差=%.3f\n", sk.Date, f.name, f.tdx, f.sina, diff)
				}
			}
		}
		if errCount == 0 {
			fmt.Printf("  %d天K线OHLC全部一致 OK\n", matchCount)
		} else {
			fmt.Printf("  %d天中%d个字段不一致\n", matchCount, errCount)
		}
	}

	// === 3. 指数K线 vs 新浪 ===
	fmt.Println("\n========== 指数K线 vs 新浪财经 ==========")
	indexCodes := []struct {
		tdxCode  string
		sinaCode string
		name     string
	}{
		{"399001", "sz399001", "深证成指"},
		{"399006", "sz399006", "创业板指"},
	}

	for _, tc := range indexCodes {
		fmt.Printf("\n--- %s (%s) ---\n", tc.tdxCode, tc.name)
		tdxKlines, err := c.GetIndexDay(tc.tdxCode, 0, 20)
		if err != nil {
			fmt.Printf("  TDX获取失败: %v\n", err)
			continue
		}
		sinaKlines := fetchSinaKline(tc.sinaCode, 20)
		if len(sinaKlines) == 0 {
			fmt.Println("  新浪K线获取失败")
			continue
		}

		tdxMap := make(map[string]*protocol.Kline)
		for _, k := range tdxKlines.List {
			tdxMap[k.Time.Format("2006-01-02")] = k
		}

		matchCount := 0
		priceErr := 0
		for _, sk := range sinaKlines {
			tk, ok := tdxMap[sk.Date]
			if !ok {
				continue
			}
			matchCount++
			if matchCount > 5 {
				break
			}
			closeDiff := tk.Close.Float64() - sk.Close
			if math.Abs(closeDiff) > 0.01 {
				priceErr++
				fmt.Printf("  [DIFF] %s 收盘: TDX=%.3f 新浪=%.3f 差=%.3f\n", sk.Date, tk.Close.Float64(), sk.Close, closeDiff)
			}
			// 指数成交量: TDX是手,新浪是股
			volRatio := float64(tk.Volume) / float64(sk.Volume)
			fmt.Printf("  %s: 收盘TDX=%.3f 新浪=%.3f 差=%.4f  量TDX=%d 量新浪=%d 量比=%.6f\n",
				sk.Date, tk.Close.Float64(), sk.Close, closeDiff, tk.Volume, sk.Volume, volRatio)
		}
		if priceErr == 0 {
			fmt.Printf("  %d天指数收盘价全部一致 OK\n", matchCount)
		}
	}

	// === 4. GetIndexDay 上证指数 panic 复现 ===
	fmt.Println("\n========== GetIndexDay 上证指数 panic 复现 ==========")
	fmt.Println("  (上证指数sh000001在之前的测试中触发了 slice bounds out of range panic)")
	fmt.Println("  panic位置: protocol/model_kline.go:189")
	fmt.Println("  原因: 指数K线解码时 bs[:4] 越界, 数据长度不足4字节")
	fmt.Println("  这是确认的Bug: 当日盘中请求指数K线时, 最后一根K线数据不完整导致panic")

	fmt.Println("\n=== 验证完成 ===")
}

// ==================== 腾讯财经API ====================
type TencentQuote struct {
	Name       string
	Open       float64
	LastClose  float64
	Price      float64
	High       float64
	Low        float64
	Volume     int
	Amount     float64
	InnerVol   int
	OuterVol   int
	BidPrices  [5]float64
	BidVols    [5]int
	AskPrices  [5]float64
	AskVols    [5]int
}

func fetchTencentQuote(code string) *TencentQuote {
	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s", code)
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
	fields := strings.Split(data, "~")

	if len(fields) < 48 {
		return nil
	}

	q := &TencentQuote{
		Name:      fields[1],
		Open:      atof(fields[5]),
		LastClose: atof(fields[4]),
		Price:     atof(fields[3]),
		High:      atof(fields[33]),
		Low:       atof(fields[34]),
		Volume:    atoi(fields[36]),
		Amount:    atof(fields[37]),
		InnerVol:  atoi(fields[26]), // 内盘(手)
		OuterVol:  atoi(fields[27]), // 外盘(手)
	}

	// 五档买盘 (fields[9-18]: 买1-5量/价)
	for i := 0; i < 5; i++ {
		q.BidVols[i] = atoi(fields[9+i*2])
		q.BidPrices[i] = atof(fields[9+i*2+1])
	}
	// 五档卖盘 (fields[19-28]: 卖1-5量/价)
	for i := 0; i < 5; i++ {
		q.AskVols[i] = atoi(fields[19+i*2])
		q.AskPrices[i] = atof(fields[19+i*2+1])
	}

	return q
}

// ==================== 新浪K线API ====================
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

// ==================== 辅助 ====================
func cmpPrice(field string, tdx, other float64) {
	diff := tdx - other
	status := "OK"
	if math.Abs(diff) > 0.005 {
		status = "DIFF"
	}
	fmt.Printf("  %-10s  %14.3f  %14.3f  %14.3f  %s\n", field, tdx, other, diff, status)
}

func atof(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func atoi(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}
