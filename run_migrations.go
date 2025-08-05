package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./data/photofield.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	files, err := ioutil.ReadDir("./db/migrations")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".up.sql") {
			continue
		}
		fmt.Println("Applying migration: ", file.Name())
		content, err := ioutil.ReadFile("./db/migrations/" + file.Name())
		if err != nil {
			log.Fatal(err)
		}
		_, err = db.Exec(string(content))
		if err != nil {
			log.Fatal(err)
		}
	}
}
