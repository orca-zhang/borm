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

	_, _ = db.Exec(`create table users(id integer primary key, name text, age integer);`)

	t := b.Table(db, "users")

	u := User{Name: "alice", Age: 20}
	_, _ = t.Insert(&u)

	u2 := User{Name: "bob", Age: 30}
	_, _ = t.Insert(&u2)

	// select with reuse
	var list []User
	_, _ = t.Reuse().Select(&list, b.Where(b.Gt("age", 18)))
	fmt.Printf("users=%+v\n", list)

	// update
	u.Age = 21
	_, _ = t.Update(&u, b.Where(b.Eq("id", u.ID)))

	// delete
	_, _ = t.Delete(b.Where(b.Eq("id", u2.ID)))
}
