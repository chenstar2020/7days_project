package main

import (
	"context"
	"fmt"
	"geerpc/registry"
	server2 "geerpc/server"
	"geerpc/xclient"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Foo int
type Args struct{Num1, Num2 int }
func (f Foo)Sum(args Args, reply *int) error{
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo)Sleep(args Args, reply *int) error{
	time.Sleep(time.Second)
	*reply = args.Num1 * args.Num2
	return nil
}

/*func (f Foo)Mul(args Args, reply *int) error{
	*reply = args.Num1 * args.Num2
	return nil
}*/


/*func startServer(addr chan string){
	var foo Foo
	if err := server.Register(&foo); err != nil {
		log.Fatal("register error:", err)
	}
	l, err := net.Listen("tcp", ":0")
	if err != nil{
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	server.Accept(l)
}

func main(){
	addr := make(chan string)
	go startServer(addr)

	client, _ := client.Dial("tcp", <-addr)
	defer func() {
		_ = client.Close()
	}()

	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for i := 1; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Num1: rand.Intn(100), Num2: rand.Intn(100)}
			var reply int
			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil{
				log.Fatal("call Foo.Sum error:", err)
			}
			fmt.Printf("%d + %d = %d\n", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}*/


//******************支持HTTP******************
/*func startServer(addr chan string){
	var foo Foo
	l, _ := net.Listen("tcp", ":0")
	_ = server.Register(&foo)
	server.HandleHTTP()
	addr <- l.Addr().String()
	_ = http.Serve(l, nil)
}

func call(addrCh chan string){
	cli, _ := client.DialHTTP("tcp", <-addrCh)
	defer func() {_ = cli.Close()}()

	time.Sleep(time.Second)

	rand.Seed(time.Now().UnixNano())
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Num1: rand.Intn(100), Num2: rand.Intn(100)}
			var reply int
			if err := cli.Call(context.Background(), "Foo.Sum", args, &reply); err != nil{
				fmt.Println("[ERROR]call Foo.Sum error:", err)
			}
			fmt.Printf("%d + %d = %d\n", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}

func main(){
	ch := make(chan string)
	go call(ch)
	startServer(ch)
}*/

//*******************负载均衡*********************

/*func startServer(addrCh chan string){
	var foo Foo
	svr := server2.NewServer()
	_ = svr.Register(&foo)

	l, _ := net.Listen("tcp", ":0")
	addrCh <- l.Addr().String()
	svr.Accept(l)
}

func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args){
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil{
		fmt.Printf("%s %s error: %v\n", typ, serviceMethod, err)
	}else{
		fmt.Printf("%s %s success: %d + %d = %d\n", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func call(addr1, addr2 string){
	d := xclient.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})  //注册服务端
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() {_ = xc.Close()}()

	var wg sync.WaitGroup
	for i := 0; i < 5;i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(addr1, addr2 string){
	d := xclient.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() {_ = xc.Close()}()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
			ctx, _ := context.WithTimeout(context.Background(), time.Second * 2)
			foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func main(){
	log.SetFlags(0)
	ch1 := make(chan string)
	ch2 := make(chan string)
	go startServer(ch1)          //开启两个服务
	go startServer(ch2)

	addr1 := <-ch1
	addr2 := <-ch2
	time.Sleep(time.Second)
	call(addr1, addr2)
	broadcast(addr1, addr2)
}*/


//*****************注册中心********************

//启动注册中心
func startRegistry(wg *sync.WaitGroup){
	l,_ := net.Listen("tcp", ":9999")
	registry.HandleHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

func startServer(registryAddr string, wg *sync.WaitGroup){
	var foo Foo
	l, _ := net.Listen("tcp", ":0")  //随机绑定端口
	server := server2.NewServer()
	_ = server.Register(&foo)
	registry.Heartbeat(registryAddr, "tcp@" + l.Addr().String(), 0)
	wg.Done()
	server.Accept(l)
}

func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args){
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil{
		fmt.Printf("%s %s error: %v\n", typ, serviceMethod, err)
	}else{
		fmt.Printf("%s %s success: %d + %d = %d\n", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func call(registry string){
	d := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() {_ = xc.Close()}()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(registry string){
	d := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() {_ = xc.Close()}()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i*i})
			ctx,_ := context.WithTimeout(context.Background(), time.Second * 2)
			foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i*i})
		}(i)
	}
	wg.Wait()
}

func main(){
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/_geerpc_/registry"
	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	time.Sleep(time.Second)
	wg.Add(2)
	go startServer(registryAddr, &wg)
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call(registryAddr)
	broadcast(registryAddr)
}