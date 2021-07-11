package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"geerpc/common"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

//Call represents an active RPC
type Call struct {
	Seq uint64
	ServiceMethod string  //format "<service>.<method>"
	Args interface{}      //函数参数
	Reply interface{}     //返回值
	Error error           //错误信息
	Done chan *Call       //支持异步调用
}

//调用结束时 会调用done通知对方
func (call *Call)done(){
	call.Done <- call
}

type Client struct {
	cc codec.Codec       //编解码器
	opt *common.Option
	sending sync.Mutex   //保证请求有序发送
	header codec.Header
	mu sync.Mutex
	seq uint64           //请求编号
	pending map[uint64]*Call  //保存待返回的请求
	closing bool         //用户主动关闭标志
	shutdown bool        //server has told us to stop
}

var ErrShutdown = errors.New("connection is shutdown")

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closing{
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client)IsAvailable()bool{
	client.mu.Lock()
	defer client.mu.Unlock()

	return !client.shutdown && !client.closing
}


var _ io.Closer = (*Client)(nil)

func (client *Client)registerCall(call *Call)(uint64, error){
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closing || client.shutdown{
		return 0, ErrShutdown
	}

	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

func (client *Client)removeCall(seq uint64)*Call{
	client.mu.Lock()
	defer client.mu.Unlock()

	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

//服务器或客户端发生错误时调用
func (client *Client)terminateCalls(err error){
	client.sending.Lock()
	defer client.sending.Unlock()

	client.mu.Lock()
	defer client.mu.Unlock()

	client.shutdown = true  //置客户端终止标志

	for _, call := range client.pending{
		call.Error = err
		call.done()
	}
}

//发送请求
func (client *Client)send(call *Call){
	client.sending.Lock()
	defer client.sending.Unlock()

	seq, err := client.registerCall(call)
	if err != nil{
		call.Error = err
		call.done()
		return
	}

	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	if err := client.cc.Write(&client.header, call.Args); err != nil{
		call := client.removeCall(seq)
		if call != nil{
			call.Error = err
			call.done()
		}
	}
}

//接收服务器的响应
func (client *Client)receive(){
	var err error
	for err == nil{
		var h codec.Header
		if err = client.cc.ReadHeader(&h);err != nil {     //此处会阻塞直到有数据返回
			break
		}

		call := client.removeCall(h.Seq)
		switch{
		case call == nil:     //call不存在
			err = client.cc.ReadBody(nil)
		case h.Error != "":   //服务端处理出错
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil{
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	client.terminateCalls(err)
}

func NewClient(conn net.Conn, opt *common.Option)(*Client, error){
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil{
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		fmt.Println("rpc client:codec error:", err)
		return nil, err
	}

	if err := json.NewEncoder(conn).Encode(opt); err != nil{  //将编码类型封装进报文
		fmt.Println("rpc client:options error:", err)
		_ = conn.Close()
		return nil, err
	}

	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *common.Option)*Client{
	client := &Client{
		seq: 1,
		cc: cc,
		opt: opt,
		pending: make(map[uint64]*Call),
	}

	go client.receive()     //开启接收响应的协程
	return client
}


//发送请求

func parseOptions(opts ...*common.Option)(*common.Option, error){  //为啥要用可变参数
	if len(opts) == 0 || opts[0] == nil{
		return common.DefaultOption, nil
	}
	if len(opts) != 1{
		return nil, errors.New("number of options is more than 1")
	}

	opt := opts[0]
	opt.MagicNumber = common.DefaultOption.MagicNumber
	if opt.CodecType == ""{
		opt.CodecType = common.DefaultOption.CodecType
	}
	return opt, nil
}

func Dial(network, address string, opts ...*common.Option)(client *Client, err error){
/*	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(network, address)
	if err != nil{
		return nil, err
	}
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()

	return NewClient(conn, opt)*/
	return dialTimeout(NewClient, network, address, opts...)
}


//对外提供的接口
func (client *Client)Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call{
	if done == nil{
		done = make(chan *Call, 10)
	}else if cap(done) == 0{
		fmt.Println("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args: args,
		Reply: reply,
		Done: done,
	}
	client.send(call)
	return call
}

func (client *Client)Call(ctx context.Context, serviceMethod string, args, reply interface{})error{
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select{
	case <-ctx.Done():
		client.removeCall(call.Seq)
		return errors.New("rpc client: call failed:" + ctx.Err().Error())
	case call := <-call.Done:
		return call.Error
	}
}


type clientResult struct{
	client *Client
	err error
}

type newClientFunc func(conn net.Conn, opt *common.Option)(client *Client, err error)


func dialTimeout(f newClientFunc, network, address string, opts ...*common.Option)(client *Client, err error){
	opt, err := parseOptions(opts...)
	if err != nil{
		return nil, err
	}
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil{
		return nil, err
	}
	defer func() {
		if err != nil{
			_ = conn.Close()
		}
	}()
	ch := make(chan clientResult)
	go func() {
		client, err := f(conn, opt)
		ch <- clientResult{client:client, err:err}
	}()
	if opt.ConnectTimeout == 0 {  //不限制等待时间 直到返回响应
		result := <-ch
		return result.client, result.err
	}
	select{
	case <-time.After(opt.ConnectTimeout):
		return nil, fmt.Errorf("rpc client:connect: expect within %s", opt.ConnectTimeout)
	case result := <-ch:
		return result.client, result.err
	}
}



//******************支持HTTP********************

func NewHTTPClient(conn net.Conn, opt *common.Option)(*Client, error){
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", common.DefaultRPCPath))

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method:"CONNECT"})
	if err == nil && resp.Status == common.Connected{
		return NewClient(conn, opt)
	}

	if err == nil{
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

func DialHTTP(network, address string, opts ...*common.Option)(*Client, error){
	return dialTimeout(NewHTTPClient, network, address, opts...)
}

func XDial(rpcAddr string, opts ...*common.Option)(*Client, error){
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2{
		return nil, fmt.Errorf("rpc client err: wrong format '%s', expect protocol@addr", rpcAddr)
	}

	protocol, addr := parts[0], parts[1]
	switch protocol{
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protocol, addr, opts...)
	}
}

