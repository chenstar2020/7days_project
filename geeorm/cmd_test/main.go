package main

import (
	"fmt"
	//_ "github.com/mattn/go-sqlite3"
	"geeorm"
)

func main(){
	engine, _ := geeorm.NewEngine("mysql", "root:root@tcp(127.0.0.1:3306)/star?charset=utf8")
	defer engine.Close()

	s := engine.NewSession()
	_, _ = s.Raw("DROP TABLE IF EXISTS User;").Exec()
	_, _ = s.Raw("CREATE TABLE User(Name text);").Exec()
	_, _ = s.Raw("CREATE TABLE User123(Name text);").Exec()

	result, _ := s.Raw("INSERT INTO User(`Name`) values(?), (?)", "Tom", "Jack").Exec()
	count, _ := result.RowsAffected()
	fmt.Printf("Exec success, %d affected\n", count)
}
