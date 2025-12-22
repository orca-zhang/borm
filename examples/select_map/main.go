package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/orca-zhang/borm"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 创建测试数据库
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 创建测试表
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT,
			age INTEGER
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 插入测试数据
	_, err = db.Exec(`
		INSERT INTO users (id, name, age) VALUES 
		(1, 'Alice', 25),
		(2, 'Bob', 30),
		(3, 'Charlie', 35)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 测试Select到Map
	table := borm.Table(db, "users").Debug()

	// 测试单条记录
	var result borm.V
	count, err := table.Select(&result, borm.Fields("id", "name", "age"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("单条记录: count=%d, result=%+v\n", count, result)

	// 测试多条记录
	var results []borm.V
	count, err = table.Select(&results, borm.Fields("id", "name", "age"), borm.Where(borm.Eq("age", 30)))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("多条记录: count=%d, results=%+v\n", count, results)
}
