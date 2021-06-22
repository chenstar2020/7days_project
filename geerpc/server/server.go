package server

import (
	"encoding/json"
	"fmt"
	"geerpc/codec"
	"geerpc/common"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

type Server struct{}

//NewServer returns a new Server
func NewServer()*Server{
	return &Server{}
}

var DefaultServer = NewServer()

//Accept accepts connections on the listener and serves requests
//for each incoming connections
func (server *Server)Accept(lis net.Listener){
	for {
		conn, err := lis.Accept()
		if err != nil{
			log.Println("rpc server: accept error:", err)
			continue
		}

		go server.ServerConn(conn)  //处理连接
	}
}

func (server *Server)ServerConn(conn io.ReadWriteCloser){
	defer func() {
		_ = conn.Close()
	}()

	var opt common.Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error:", err)
		return
	}
	//校验请求类型
	if opt.MagicNumber != common.MagicNumber{
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	//得到解码器
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil{
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	//开始解码数据
	server.serverCodec(f(conn))
}

var invalidRequest = struct {

}{}

func (server *Server)serverCodec(cc codec.Codec){
	sending := new(sync.Mutex)  //make sure to send a complete request
	wg := new(sync.WaitGroup)

	for {
		req, err := server.readRequest(cc)
		if err != nil{
			if req == nil{
				break   //读取数据失败  关闭连接并退出协程
			}
			req.h.Error = err.Error()  //把错误原因传给客户端
			server.sendResponse(cc, req.h, req.argv, sending)
			continue
		}
		wg.Add(1)       //请求method可以有多个
		go server.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()   //等待所有请求处理完成
	_ = cc.Close()
}


type request struct {
	h *codec.Header              //header
	argv, replyv reflect.Value   //body
}

//读取header
func (server *Server)readRequestHeader(cc codec.Codec)(*codec.Header, error){
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF{
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}


func (server *Server)readRequest(cc codec.Codec)(*request, error){
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h:h}
	//读取body
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil{
		log.Println("rpc server: read argv err:", err)
	}
	return req, nil
}

func (server *Server)sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex){
	sending.Lock()
	defer sending.Unlock()   //串行发送响应 防止数据混乱

	if err := cc.Write(h, body); err != nil{
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server)handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup){
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())  //暂时先打印出来  还要正式调用接口处理
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

func Accept(lis net.Listener){
	DefaultServer.Accept(lis)
}