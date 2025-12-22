package main

import (
	"database/sql"
	"fmt"
	"log"

	b "github.com/orca-zhang/borm"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID   int64  `borm:"id"`
	Name string `borm:"name"`
	Age  int    `borm:"age"`
}

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, _ = db.Exec(`create table users(id integer primary key, name text, age integer);
	insert into users(id,name,age) values (1,'alice',20),(2,'bob',30);`)

	t := b.Table(db, "users")

	var u User
	n, err := t.Select(&u, b.Where(b.Eq("id", 1)))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("rows=%d user=%+v\n", n, u)

	var ids []int64
	n, err = t.Select(&ids, b.Fields("id"), b.Where(b.Gt("age", 18)))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("rows=%d ids=%v\n", n, ids)
}
