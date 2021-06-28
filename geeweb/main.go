package main

import (
	"fmt"
	"geeweb/gee"
	"html/template"
	"net/http"
	"time"
)

func onlyForV2()gee.HandlerFunc{
	return func(ctx *gee.Context) {
		//t := time.Now()
		fmt.Println(500, "Internal Server Error")
	}
}

type student struct {
	Name string
	Age int
}

func FormatAsDate(t time.Time)string{
	year, month, day := t.Date()
	return fmt.Sprintf("%d-%02d-%02d", year, month, day)
}

func main(){
	r := gee.New()
	r.Use(gee.Logger())
	r.SetFuncMap(template.FuncMap{
		"FormatAsDate": FormatAsDate,
	})
	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "/usr/geektutu/blog/static")

	stu1 := &student{Name: "Geektutu", Age: 20}
	stu2 := &student{Name: "Jack", Age: 22}

	r.GET("/index", func(ctx *gee.Context) {
		ctx.HTML(http.StatusOK, "css.tmpl", nil)
	})

	r.GET("/students", func(ctx *gee.Context) {
		ctx.HTML(http.StatusOK, "arr.tmpl", gee.H{
			"title":"gee",
			"stuArr":[2]*student{stu1, stu2},
		})
	})
	v1 := r.Group("/v1")
	{
/*		v1.GET("/", func(ctx *gee.Context) {
			ctx.HTML(http.StatusOK, "<h1>Hello Gee</h1>")
		})
*/
		v1.GET("/hello", func(ctx *gee.Context) {
			ctx.String(http.StatusOK, "hello world")
		})
	}
	
	v2 := r.Group("/v2")
	v2.Use(onlyForV2())
	{
		v2.GET("/hello/:name", func(ctx *gee.Context) {
			ctx.String(http.StatusOK, "hello %s, you're at %s\n", ctx.Param("name"), ctx.Path)
		})
	}

	r.GET("/panic", func(c *gee.Context) {
		names := []string{"geektutu"}
		c.String(http.StatusOK, names[100])
	})

	r.Run(":9999")
}
