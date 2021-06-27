package main

import (
	"context"
	"fmt"
	"geerpc/client"
	"geerpc/server"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Foo int
type Args struct{Num1, Num2 int }
func (f Foo)Sum(args Args, reply *int) error{
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo)Mul(args Args, reply *int) error{
	*reply = args.Num1 * args.Num2
	return nil
}

func startServer(addr chan string){
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
}
