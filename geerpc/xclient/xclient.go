package xclient

import (
	"context"
	"geerpc/client"
	"geerpc/common"
	"io"
	"reflect"
	"sync"
)

type XClient struct {
	d Discovery               //服务器注册和发现中心
	mode SelectMode
	opt *common.Option
	mu sync.Mutex
	clients map[string]*client.Client
}

func NewXClient(d Discovery, mode SelectMode, opt *common.Option)*XClient{
	return &XClient{d: d, mode: mode, opt: opt, clients: make(map[string]*client.Client)}
}

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	for key, client := range xc.clients{
		_ = client.Close()
		delete(xc.clients, key)
	}
	return nil
}

var _ io.Closer = (*XClient)(nil)


func (xc *XClient)dial(rpcAddr string)(*client.Client, error){
	xc.mu.Lock()
	defer xc.mu.Unlock()
	xclient, ok := xc.clients[rpcAddr]
	if ok && !xclient.IsAvailable(){
		_ = xclient.Close()
		delete(xc.clients, rpcAddr)
		xclient = nil
	}
	if xclient == nil {
		var err error
		xclient, err = client.XDial(rpcAddr, xc.opt)
		if err != nil{
			return nil, err
		}
		xc.clients[rpcAddr] = xclient
	}
	return xclient, nil
}

func (xc *XClient)call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{} )error{
	client1, err := xc.dial(rpcAddr)  //选择服务器
	if err != nil{
		return err
	}
	return client1.Call(ctx, serviceMethod, args, reply)
}

func (xc *XClient)Call(ctx context.Context, serviceMethod string, args, reply interface{})error{
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil{
		return err
	}
	return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}

func (xc *XClient)Broadcast(ctx context.Context, serviceMethod string, args, reply interface{})error{
	servers, err := xc.d.GetAll()
	if err != nil{
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error
	replyDone := reply == nil
	ctx, cancel := context.WithCancel(ctx)
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var clonedReply interface{}
			if reply != nil{
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr, ctx, serviceMethod, args, clonedReply)
			mu.Lock()
			if err != nil && e == nil{
				e = err
				cancel()
			}
			if err == nil && !replyDone{
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return e
}
