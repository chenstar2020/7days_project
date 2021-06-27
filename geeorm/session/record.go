package session

import (
	"geeorm/clause"
	"reflect"
)

func (s *Session)Insert(values...interface{})(int64, error){
	recordValues := make([]interface{}, 0)
	for _, value := range values{
		table := s.Model(value).RefTable()
		s.clause.Set(clause.INSERT, table.Name, table.FieldNames)
		recordValues = append(recordValues, table.RecordValues(value))
	}

	s.clause.Set(clause.VALUES, recordValues...)
	sql, valrs := s.clause.Build(clause.INSERT, clause.VALUES)
	result, err := s.Raw(sql, valrs...).Exec()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Session)Find(values interface{})error{
	s.CallMethod(BeforeQuery, nil)
	destSlice := reflect.Indirect(reflect.ValueOf(values))
	destType := destSlice.Type().Elem()
	table := s.Model(reflect.New(destType).Elem()).RefTable()

	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	sql, vars := s.clause.Build(clause.SELECT, clause.WHERE, clause.ORDERBY, clause.LIMIT)
	rows, err := s.Raw(sql, vars...).QueryRows()
	if err != nil{
		return err
	}

	for rows.Next(){
		dest := reflect.New(destType).Elem()
		var values []interface{}
		for _, name := range table.FieldNames{
			values = append(values, dest.FieldByName(name).Addr().Interface())
		}
		if err := rows.Scan(values...); err != nil{
			return err
		}
		s.CallMethod(AfterQuery, dest.Addr().Interface())
		destSlice.Set(reflect.Append(destSlice, dest))
	}
	return rows.Close()
}

func (s *Session)Update(kv...interface{})(int64, error){
	m, ok := kv[0].(map[string]interface{})
	if !ok{
		m = make(map[string]interface{})
		for i := 0; i < len(kv); i += 2{ //为啥是加2
			m[kv[i].(string)] = kv[i + 1]
		}
	}

	s.clause.Set(clause.UPDATE, s.RefTable().Name, m)
	sql, vars := s.clause.Build(clause.UPDATE, clause.WHERE)
	result, err := s.Raw(sql, vars...).Exec()
	if err != nil{
		return 0, err
	}
	return result.RowsAffected()
}