package tdx

import (
	"errors"
	"time"

	"github.com/injoyai/base/safe"
	"github.com/injoyai/logs"
)

type (
	IPool interface {
		Get() (*Client, error)
		Put(c *Client)
		Do(fn func(c *Client) error) error
		Go(fn func(c *Client)) error
	}
	DialPoolFunc = func() (IPool, error)
)

// NewPool 简易版本的连接池
func NewPool(dial func() (*Client, error), number int) (*Pool, error) {
	if number <= 0 {
		number = 1
	}
	ch := make(chan *Client, number)
	p := &Pool{
		ch:   ch,
		dial: dial,
		Closer: safe.NewCloser().SetCloseFunc(func(err error) error {
			close(ch)
			return nil
		}),
	}
	for i := 0; i < number; i++ {
		c, err := dial()
		if err != nil {
			return nil, err
		}
		p.ch <- c
	}
	return p, nil
}

type Pool struct {
	ch   chan *Client
	dial func() (*Client, error) // 连接创建函数，用于重建失效连接
	*safe.Closer
}

func (this *Pool) Get() (*Client, error) {
	for {
		select {
		case <-this.Done():
			return nil, this.Err()
		case c, ok := <-this.ch:
			if !ok {
				return nil, errors.New("已关闭")
			}
			// 健康检查：内部 client.Client 为 nil，需重建
			if c.Client == nil {
				logs.Warnf("连接池发现无效连接，正在重建...")
				if this.dial != nil {
					if newClient, err := this.dial(); err == nil {
						return newClient, nil
					}
				}
				continue
			}
			// 健康检查：全局生命周期结束，需重建
			select {
			case <-c.Done():
				logs.Warnf("连接池发现已关闭连接，正在重建...")
				if this.dial != nil {
					if newClient, err := this.dial(); err == nil {
						return newClient, nil
					}
				}
				continue
			default:
			}
			// 健康检查：单次连接断开，等待重连或重建
			if c.Closed() {
				select {
				case <-c.Dialed():
					return c, nil
				case <-time.After(5 * time.Second):
					logs.Warnf("连接池发现断开连接，正在重建...")
					if this.dial != nil {
						if newClient, err := this.dial(); err == nil {
							return newClient, nil
						}
					}
					continue
				}
			}
			return c, nil
		}
	}
}

func (this *Pool) Put(c *Client) {
	select {
	case <-this.Done():
		c.Close()
		return
	default:
	}
	// 状态验证：内部 client.Client 为 nil 或已关闭的连接不放回池中
	if c.Client == nil {
		logs.Warnf("连接池丢弃无效连接")
		return
	}
	select {
	case <-c.Done():
		logs.Warnf("连接池丢弃已断开连接")
		return
	default:
	}
	if c.Closed() {
		logs.Warnf("连接池丢弃已断开连接")
		return
	}
	select {
	case this.ch <- c:
	default:
		c.Close()
	}
}

func (this *Pool) Do(fn func(c *Client) error) error {
	c, err := this.Get()
	if err != nil {
		return err
	}
	defer this.Put(c)
	return fn(c)
}

func (this *Pool) Go(fn func(c *Client)) error {
	c, err := this.Get()
	if err != nil {
		return err
	}
	go func(c *Client) {
		defer this.Put(c)
		fn(c)
	}(c)
	return nil
}
