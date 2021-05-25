package singleflight

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestGroup_Do(t *testing.T) {
	gg := &Group{
		mu: sync.Mutex{},
		m: make(map[string]*call),
	}


	for i := 0; i < 10; i++ {
		go gg.Do("hello", func() (interface{}, error) {
			time.Sleep(time.Second * 2)
			fmt.Println("callback")
			return "key" + strconv.Itoa(i), nil
		})
	}


	time.Sleep(time.Second * 30)

}
