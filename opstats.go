package main

import (
	"database/sql"
	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
	_ "github.com/lib/pq"
	"os"
	"strings"
	"time"
	//"fmt"
)

func main() {
	db := dbSetup()
	defer db.Close()

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	m := martini.Classic()
	m.Use(render.Renderer(render.Options{
		Layout: "layout",
	}))

	m.Get("/", func(r render.Render) {
		ql, err := getRunningQueries(db)
		if err != nil {
			panic(err)
		}
		r.HTML(200, "opstats", &opstatPkg{QueryList: ql, Hostname: hostname})
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

// A convenience type to feed templates with.
type opstatPkg struct {
	QueryList activeQueryList
	Hostname  string
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

func (aq *activeQuery) Duration() time.Duration {
	return time.Since(aq.StartTime)
}

type activeQueryList []activeQuery

func (aql *activeQueryList) SessionCount() int {
	return len(*aql)
}

func (aql *activeQueryList) IdleCount() int {
	count := 0
	for _, v := range *aql {
		if v.NotSetOperation() {
			continue
		}
		count = count + 1
	}
	return count
}

func (aql *activeQueryList) ActiveCount() int {
	return len(*aql) - aql.IdleCount()
}
