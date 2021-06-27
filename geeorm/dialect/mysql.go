package dialect

import (
	"fmt"
	"reflect"
	"time"
)

type mysql struct{}

var _MDialect = (*mysql)(nil)

func init(){
	RegisterDialect("mysql", &mysql{})
}

func (m mysql) DataTypeOf(typ reflect.Value) string { //golang类型和sqlite类型转换
	switch typ.Kind(){
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
		return "integer"
	case reflect.Int64, reflect.Uint64:
		return "bigint"
	case reflect.Float32, reflect.Float64:
		return "real"
	case reflect.String:
		return "text"
	case reflect.Array, reflect.Slice:
		return "blob"
	case reflect.Struct:
		if _, ok := typ.Interface().(time.Time); ok {
			return "datetime"
		}
	}
	panic(fmt.Sprintf("invalid sql type %s (%s)", typ.Type().Name(), typ.Kind()))
}

func (m mysql) TableExistSQL(tableName string) (string, []interface{}) {
	args := []interface{}{tableName}
	return "SELECT * FROM information_schema.tables where table_name = ?", args
}

