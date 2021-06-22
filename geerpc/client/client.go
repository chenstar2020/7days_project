package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"geerpc/common"
	"io"
	"log"
	"net"
	"sync"
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
	pending map[uint64]*Call
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
		if err = client.cc.ReadHeader(&h);err != nil {
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
		log.Println("rpc client:codec error:", err)
		return nil, err
	}

	if err := json.NewEncoder(conn).Encode(opt); err != nil{  //将编码类型封装进报文
		log.Println("rpc client:options error:", err)
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
	opt, err := parseOptions(opts...)
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

	return NewClient(conn, opt)
}


//对外提供的接口
func (client *Client)Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call{
	if done == nil{
		done = make(chan *Call, 10)
	}else if cap(done) == 0{
		log.Panic("rpc client: done channel is unbuffered")
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

func (client *Client)Call(serviceMethod string, args, reply interface{})error{
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
