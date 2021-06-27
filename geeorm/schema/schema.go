package schema

import (
	"geeorm/dialect"
	"go/ast"
	"reflect"
)

type Field struct {
	Name string
	Type string
	Tag  string
}

type Schema struct {
	Model interface{}  //映射对象
	Name  string       //表名
	Fields []*Field    //字段
	FieldNames []string
	fieldMap map[string]*Field
}

func (schema *Schema)GetField(name string)*Field{
	return schema.fieldMap[name]
}

func Parse(dest interface{}, d dialect.Dialect) *Schema{
	modelType := reflect.Indirect(reflect.ValueOf(dest)).Type()  //reflect.Indirect 获取指针指向的实例
	schema := &Schema{
		Model: dest,
		Name: modelType.Name(),
		fieldMap: make(map[string]*Field),
	}

	for i := 0; i < modelType.NumField(); i++ {
		p := modelType.Field(i)
		if !p.Anonymous && ast.IsExported(p.Name){
			field := &Field{
				Name: p.Name,
				Type: d.DataTypeOf(reflect.Indirect(reflect.New(p.Type))),
			}
			if v, ok := p.Tag.Lookup("geeorm"); ok {
				field.Tag = v
			}
			schema.Fields = append(schema.Fields, field)
			schema.FieldNames = append(schema.FieldNames, p.Name)
			schema.fieldMap[p.Name] = field
		}
	}
	return schema
}

func (schema *Schema)RecordValues(dest interface{})[]interface{}{
	destValue := reflect.Indirect(reflect.ValueOf(dest))
	var fieldValues []interface{}
	for _, field := range schema.Fields{
		fieldValues = append(fieldValues, destValue.FieldByName(field.Name).Interface())
	}
	return fieldValues
}