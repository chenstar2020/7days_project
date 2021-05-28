package main

import (
	"geeweb/gee"
	"net/http"
)


func main(){
	r := gee.New()
	r.GET("/", func(c *gee.Context){
		c.HTML(http.StatusOK, "<h1>Hello Gee</h1>")
	})

	r.GET("/hello", func(c *gee.Context){
		c.String(http.StatusOK, "hello %s, path:%s", c.Query("name"), c.Path)
	})

	r.POST("/login", func(c *gee.Context){
		c.JSON(http.StatusOK, gee.H{
			"username": c.PostForm("username"),
			"password": c.PostForm("password"),
		})
	})
	r.Run(":9999")
}
