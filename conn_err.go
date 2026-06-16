package tdx

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/injoyai/logs"
)

// isConnErr 判断错误是否为网络连接相关错误
func isConnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, keyword := range []string{
		"use of closed network connection",
		"operation timed out",
		"connection reset by peer",
		"broken pipe",
	} {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// ensureClient 确保 Client 连接可用，必要时重建连接
// 参数 c 为 Client 指针的指针，重建时会替换为新实例
// 参数 dial 为连接创建函数
func ensureClient(c **Client, dial DialClientFunc) error {
	if *c == nil || (*c).Client == nil {
		newClient, err := dial()
		if err != nil {
			return err
		}
		*c = newClient
		return nil
	}

	select {
	case <-(*c).Done():
		// 全局生命周期结束，需重建
		logs.Warnf("客户端连接已关闭，正在重建...")
		newClient, err := dial()
		if err != nil {
			return err
		}
		*c = newClient
		return nil
	default:
	}

	if (*c).Closed() {
		// 单次连接断开，等待自动重连
		select {
		case <-(*c).Dialed():
			logs.Warnf("客户端自动重连成功")
			return nil
		case <-time.After(30 * time.Second):
			// 重连超时，手动重建
			logs.Warnf("客户端自动重连超时，正在手动重建...")
			newClient, err := dial()
			if err != nil {
				return err
			}
			*c = newClient
			return nil
		}
	}

	return nil
}
