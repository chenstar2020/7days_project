package gee

import (
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"
)

type HandlerFunc func(ctx *Context)

type Engine struct {
	*RouterGroup          //这个用来直接调用全匹配路径函数
	router *router
	groups []*RouterGroup //store all groups

	htmlTemplates *template.Template
	funcMap template.FuncMap
}

type RouterGroup struct{
	prefix string
	middlewares []HandlerFunc // support middleware
	parent *RouterGroup //support nesting
	engine *Engine
}

func New()*Engine{
	 engine := &Engine{
		router: newRouter(),
	 }
	 engine.RouterGroup = &RouterGroup{
	 	engine: engine,    //循环调用？？
	 }
	 engine.groups = []*RouterGroup{
	 	engine.RouterGroup,
	 }
	 return engine
}

func (engine *Engine)SetFuncMap(funcMap template.FuncMap){
	engine.funcMap = funcMap
}

func (engine *Engine)LoadHTMLGlob(pattern string){
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
}

func (group *RouterGroup)Group(prefix string)*RouterGroup{
	engine := group.engine
	newGroup := &RouterGroup{
		prefix: group.prefix + prefix,
		parent: group,
		engine: engine,
	}
	engine.groups = append(engine.groups, newGroup)
	return newGroup
}

func (group *RouterGroup)addRoute(method string, comp string, handler HandlerFunc){
	pattern := group.prefix + comp
	group.engine.router.addRoute(method, pattern, handler)
}

func (group *RouterGroup)GET(pattern string, handler HandlerFunc){
	group.addRoute("GET", pattern, handler)
}

func (group *RouterGroup)POST(pattern string, handler HandlerFunc){
	group.addRoute("POST", pattern, handler)
}

func (group *RouterGroup)Run(addr string)(err error){
	return http.ListenAndServe(addr, group.engine)
}

func (group *RouterGroup)Use(middlewares...HandlerFunc){
	group.middlewares = append(group.middlewares, middlewares...)
}

func (group *RouterGroup)createStaticHandler(relativePath string, fs http.FileSystem)HandlerFunc{
	absolutePath := path.Join(group.prefix, relativePath)
	fmt.Println("absolutePath:", absolutePath)
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))
	return func(ctx *Context) {
		file := ctx.Param("filepath")
		if _, err := fs.Open(file); err != nil {
			ctx.Status(http.StatusNotFound)
			return
		}

		fileServer.ServeHTTP(ctx.Writer, ctx.Req)
	}
}

func (group *RouterGroup)Static(relativePath string, root string){
	fmt.Println("relativePath:", relativePath, "root:", root)
	handler := group.createStaticHandler(relativePath, http.Dir(root))
	urlPattern := path.Join(relativePath, "/*filepath")
	fmt.Println("urlPattern:", urlPattern)
	group.GET(urlPattern, handler)
}

func (engine *Engine)ServeHTTP(w http.ResponseWriter, r *http.Request){
	var middlewares []HandlerFunc
	for _, group := range engine.groups{
		if strings.HasPrefix(r.URL.Path, group.prefix){
			middlewares = append(middlewares, group.middlewares...)
		}
	}
	c := newContext(w, r)
	c.handlers = middlewares
	c.engine = engine
	engine.router.handle(c)
}