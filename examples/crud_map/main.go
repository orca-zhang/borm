package main

import (
	"database/sql"
	"fmt"
	"log"

	b "borm"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, _ = db.Exec(`create table users(id integer primary key, name text, age integer);`)

	t := b.Table(db, "users")

	// insert with map
	m := b.V{"name": "alice", "age": 20}
	_, _ = t.Insert(m)

	// select to []map
	var rows []b.V
	_, _ = t.Select(&rows, b.Fields("id", "name", "age"), b.Where(b.Gt("age", 18)))
	fmt.Printf("rows=%v\n", rows)

	// update with partial fields
	_, _ = t.Update(b.V{"name": "alice2", "age": 21}, b.Fields("name", "age"), b.Where(b.Eq("id", rows[0]["id"])))

	// delete
	_, _ = t.Delete(b.Where(b.Eq("id", rows[0]["id"])))
}
