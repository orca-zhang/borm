package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/orca-zhang/borm"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID   int    `borm:"id"`
	Name string `borm:"name"`
	Age  int    `borm:"age"`
}

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

	table := borm.Table(db, "users").Debug()

	// 测试Insert with Reuse
	fmt.Println("=== 测试Insert with Reuse ===")
	user1 := &User{ID: 1, Name: "Alice", Age: 25}
	count, err := table.Insert(user1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Insert 1: count=%d\n", count)

	user2 := &User{ID: 2, Name: "Bob", Age: 30}
	count, err = table.Insert(user2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Insert 2: count=%d\n", count)

	// 测试Update with Reuse
	fmt.Println("\n=== 测试Update with Reuse ===")
	user1.Age = 26
	count, err = table.Update(user1, borm.Where(borm.Eq("id", user1.ID)))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Update 1: count=%d\n", count)

	// 测试Delete with Reuse
	fmt.Println("\n=== 测试Delete with Reuse ===")
	count, err = table.Delete(borm.Where(borm.Eq("id", user2.ID)))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Delete 1: count=%d\n", count)

	// 验证结果
	fmt.Println("\n=== 验证结果 ===")
	var users []User
	count, err = table.Select(&users, borm.Fields("id", "name", "age"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("剩余用户: count=%d, users=%+v\n", count, users)
}
