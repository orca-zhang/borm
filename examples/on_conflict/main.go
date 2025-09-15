package main

import (
	"database/sql"
	"log"

	b "borm"

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

	_, _ = db.Exec(`create table users(id integer primary key, name text, age integer);`)

	t := b.Table(db, "users")

	u := User{ID: 1, Name: "alice", Age: 20}
	_, _ = t.Insert(&u, b.Fields("id", "name", "age"))

	u2 := User{ID: 1, Name: "alice2", Age: 21}
	_, _ = t.Insert(&u2, b.Fields("id", "name", "age"), b.OnConflictDoUpdateSet([]string{"id"}, b.V{
		"name": "alice2",
		"age":  b.U("age+1"),
	}))
}
