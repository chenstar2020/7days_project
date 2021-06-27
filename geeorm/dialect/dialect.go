package dialect

import "reflect"
/*
 *定义转换接口 和注册、实现函数
 */
var dialectsMap = map[string]Dialect{}

type Dialect interface {
	DataTypeOf(typ reflect.Value) string
	TableExistSQL(tableName string)(string, []interface{})
}

func RegisterDialect(name string, dialect Dialect){
	dialectsMap[name] = dialect
}

func GetDialect(name string)(dialect Dialect, ok bool){
	dialect, ok = dialectsMap[name]
	return
}