package main

import (
	"database/sql"
	"fmt"
	"log"

	b "borm"

	_ "github.com/mattn/go-sqlite3"
)

type Info struct {
	ID   int64  `borm:"t_usr.id"`
	Name string `borm:"t_usr.name"`
	Tag  string `borm:"t_tag.tag"`
}

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, _ = db.Exec(`create table t_usr(id integer primary key, name text);
	create table t_tag(id integer primary key, tag text);
	insert into t_usr(id,name) values(1,'alice');
	insert into t_tag(id,tag) values(1,'vip');`)

	t := b.Table(db, "t_usr")
	var o Info
	_, _ = t.Select(&o, b.Join("join t_tag on t_usr.id=t_tag.id"), b.Where(b.Eq("t_usr.id", 1)))
	fmt.Printf("info=%+v\n", o)
}
