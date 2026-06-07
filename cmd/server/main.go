package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/injoyai/tdx/server"
)

func main() {
	// 加载 .env 文件
	loadEnv()

	port := flag.Int("port", envInt("TDX_PORT", 8080), "服务端口")
	dir := flag.String("dir", envStr("TDX_DIR", "data"), "数据目录")
	noWeb := flag.Bool("no-web", false, "禁用Web测试页面")
	dbType := flag.String("db-type", envStr("DB_TYPE", "mysql"), "数据库类型: mysql/sqlite")
	dbDSN := flag.String("db-dsn", envStr("DB_DSN", ""), "数据库DSN(mysql时必填)")
	flag.Parse()

	// 如果没有直接指定DSN,从分项配置拼接
	dsn := *dbDSN
	if dsn == "" && *dbType == "mysql" {
		dsn = buildDSN()
	}

	if err := server.Run(server.Config{
		Port:   *port,
		Dir:    *dir,
		NoWeb:  *noWeb,
		DBType: *dbType,
		DBDSN:  dsn,
	}); err != nil {
		fmt.Printf("服务启动失败: %v\n", err)
	}
}

// buildDSN 从环境变量拼接MySQL DSN
func buildDSN() string {
	user := envStr("DB_USER", "tdx")
	password := envStr("DB_PASSWORD", "tdx123456")
	host := envStr("DB_HOST", "mysql")
	port := envInt("DB_PORT", 3306)
	name := envStr("DB_NAME", "tdx")
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		user, password, host, port, name)
}

// loadEnv 从 .env 文件加载环境变量(不覆盖已有)
func loadEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// 不覆盖已有环境变量
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
