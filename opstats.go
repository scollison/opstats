package main

import (
	"database/sql"
	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
	_ "github.com/lib/pq"
	"strings"
	"time"
	//"fmt"
)

func main() {
	db := dbSetup()
	defer db.Close()

	ql, err := getRunningQueries(db)
	if err != nil {
		panic(err)
	}

	m := martini.Classic()
	m.Use(render.Renderer(render.Options{
		Layout: "layout",
	}))

	m.Get("/", func(r render.Render) {
		r.HTML(200, "opstats", ql)
	})

	m.Run()
}

func dbSetup() *sql.DB {
	db, err := sql.Open("postgres", "sslmode=disable")
	if err != nil {
		panic(err)
	}

	return db
}

func getRunningQueries(db *sql.DB) (activeQueryList, error) {
	// Get the running queries.
	queries, err := db.Query(`select pid, usename, substring(query,1,60), 
	waiting, backend_start from pg_stat_activity order by 5 desc`)
	if err != nil {
		panic(err)
	}

	q := activeQuery{}
	ql := activeQueryList{}

	for queries.Next() {
		if err := queries.Scan(&q.Pid, &q.UserName, &q.Query,
			&q.Waiting, &q.StartTime); err != nil {
			return activeQueryList{}, err
		}

		if err = queries.Err(); err != nil {
			return activeQueryList{}, err
		}

		// Replace newlines with spaces in SQL Query string
		q.Query = strings.Replace(q.Query, "\n", " ", -1)
		q.Query = strings.Replace(q.Query, "\r", " ", -1)

		ql = append(ql, q)
	}

	err = queries.Close()
	if err != nil {
		panic(err)
	}

	return ql, nil
}

type activeQuery struct {
	Pid       int
	UserName  string
	Query     string
	Waiting   bool
	StartTime time.Time
}

func (aq *activeQuery) NotSetOperation() bool {
	if strings.HasPrefix(aq.Query, "SET ") {
		return false
	}
	return true
}

type activeQueryList []activeQuery

func (aql *activeQueryList) SessionCount() int {
	return len(*aql)
}
