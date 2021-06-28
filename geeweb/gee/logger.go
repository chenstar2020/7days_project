package gee

import (
	"fmt"
	"time"
)

func Logger() HandlerFunc{
	return func(c *Context){
		t := time.Now()
		c.Next()
		fmt.Printf("star.chen [%d] %s in %v\n", c.StatusCode, c.Req.RequestURI, time.Since(t))
	}
}
