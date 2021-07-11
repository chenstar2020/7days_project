package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"geerpc/common"
	"go/ast"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct{
	serviceMap sync.Map
}

//NewServer returns a new Server
func NewServer()*Server{
	return &Server{}
}

var DefaultServer = NewServer()

//Register publishes in the server the set of methods of the
func (server *Server)Register(rcvr interface{})error{
	s := newService(rcvr)
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup{  //要放在newService之前吧
		return errors.New("rpc: service already defined:" + s.name)
	}
	return nil
}

func (server *Server)findService(serviceMethod string)(svc *service, mtype *methodType, err error){
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed:" + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := server.serviceMap.Load(serviceName)
	if !ok{
		err = errors.New("rpc server: cant't find service " + serviceName)
		return
	}

	svc = svci.(*service)
	mtype = svc.method[methodName]
	if mtype == nil{
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}

func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

//Accept accepts connections on the listener and serves requests
//for each incoming connections
func (server *Server)Accept(lis net.Listener){
	for {
		conn, err := lis.Accept()
		if err != nil{
			fmt.Println("rpc server: accept error:", err)
			continue
		}

		go server.ServeConn(conn)  //处理连接
	}
}

func (server *Server)ServeConn(conn io.ReadWriteCloser){
	defer func() {
		_ = conn.Close()
	}()

	var opt common.Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		fmt.Println("rpc server: options error:", err)
		return
	}

	//校验请求类型
	if opt.MagicNumber != common.MagicNumber{
		fmt.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	//得到解码器
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil{
		fmt.Printf("rpc server: invalid codec type %s", opt.CodecType)
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
		go server.handleRequest(cc, req, sending, wg, 3 * time.Second)
	}
	wg.Wait()   //等待所有请求处理完成
	_ = cc.Close()
}


type request struct {
	h *codec.Header              //header
	argv, replyv reflect.Value   //body
	mtype *methodType
	svc   *service
}


//读取header
func (server *Server)readRequestHeader(cc codec.Codec)(*codec.Header, error){
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF{
			fmt.Println("rpc server: read header error:", err)
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

	req.svc, req.mtype, err = server.findService(h.ServiceMethod)        //找到处理函数
	if err != nil{
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()

	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr{
		argvi = req.argv.Addr().Interface()
	}

	if err = cc.ReadBody(argvi); err != nil{
		fmt.Println("rpc server: read body err:", err)
		return req, err
	}

	return req, nil
}

func (server *Server)sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex){
	sending.Lock()
	defer sending.Unlock()   //串行发送响应 防止数据混乱

	if err := cc.Write(h, body); err != nil{
		fmt.Println("rpc server: write response error:", err)
	}
}

func (server *Server)handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration){
	defer wg.Done()
	called := make(chan struct{})
	sent := make(chan struct{})

	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)  //调用计算函数
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sending)
			sent <- struct{}{}
			return
		}
		server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
		sent <- struct{}{}
	}()

	if timeout == 0{
		<-called
		<-sent
		return
	}

	select{
	case <-time.After(timeout): //调用函数或者发送响应的地方阻塞了
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		server.sendResponse(cc, req.h, invalidRequest, sending)
	case <-called:
		<-sent
	}
}

func Accept(lis net.Listener){
	DefaultServer.Accept(lis)
}

type methodType struct {
	method reflect.Method    //保存方法
	ArgType reflect.Type     //请求参数
	ReplyType reflect.Type   //返回参数
	numCalls uint64          //被调用次数
}

func (m *methodType)NumCalls()uint64{
	return atomic.LoadUint64(&m.numCalls)      //原子加载指针指向的值
}

func (m *methodType)newArgv()reflect.Value{
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr{
		argv = reflect.New(m.ArgType.Elem())
	}else{
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *methodType)newReplyv()reflect.Value{
	replyv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind(){
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}


type service struct {
	name string            //结构体名称
	typ reflect.Type       //结构体类型
	rcvr reflect.Value     //结构体实例本身
	method map[string]*methodType
}

func newService(rcvr interface{})*service{
	s := new(service)
	//解析参数
	s.rcvr = reflect.ValueOf(rcvr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(rcvr)
	if !ast.IsExported(s.name){
		fmt.Printf("rpc server: %s is not a valid service name",s.name)
	}
    s.registerMethods()
	return s
}

func (s *service)registerMethods(){
	s.method = make(map[string]*methodType)

	for i := 0;i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type
		if mType.NumIn() != 3 || mType.NumOut() != 1{
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem(){  //返回值是否为error
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)

		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType){
			continue
		}
		s.method[method.Name] = &methodType{
			method: method,
			ArgType: argType,
			ReplyType: replyType,
		}
		fmt.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

func isExportedOrBuiltinType(t reflect.Type)bool{
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func (s *service)call(m *methodType, argv, replyv reflect.Value) error{
	atomic.AddUint64(&m.numCalls, 1)      //原子操作
	f := m.method.Func

	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil{
		return errInter.(error)
	}
	return nil
}




//********************支持HTTP***********************

func (server *Server)ServeHTTP(w http.ResponseWriter, req *http.Request){
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil{
		fmt.Print("rpc hijacking", req.RemoteAddr, ": ", err.Error())
		return
	}

	_, _ = io.WriteString(conn, "HTTP/1.0 " + common.Connected + "\n\n")      //这里HTTP/1.0之后的空格很重要
	server.ServeConn(conn)
}

func (server *Server)HandleHTTP(){
	http.Handle(common.DefaultRPCPath, server)
	http.Handle(common.DefaultDebugPath, debugHTTP{server})
	fmt.Println("rpc server debug path:", common.DefaultDebugPath)
}

func HandleHTTP(){
	DefaultServer.HandleHTTP()
}
