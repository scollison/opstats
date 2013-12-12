package main

import (
	_ "github.com/lib/pq"
	"database/sql"
	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
	"os"
	"strings"
	"time"
)

func main() {
	// A classic martini that uses templates/layout.tmpl
	m := martini.Classic()
	m.Use(render.Renderer(render.Options{
		Layout: "layout",
	}))

	hostname, _ := os.Hostname()

	db, err := dbSetup("dbname=postgres user=postgres host=/tmp sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	m.Get("/", func(r render.Render) (int, string) {
		ll, err := getLockList(db)
		if err != nil {
			return 500, err.Error()
		}

		ql, err := getRunningQueries(db)
		if err != nil {
			return 500, err.Error()
		}

		r.HTML(200, "opstats", &opstatPkg{QueryList: ql, LockList: ll, Hostname: hostname})

		return 200, "No Error"
	})

	m.Run()
}

func dbSetup(options string) (*sql.DB, error) {
	db, err := sql.Open("postgres", options)
	if err != nil {
		return db, err
	}

	return db, err
}

func getRunningQueries(db *sql.DB) (activeQueryList, error) {
	// Get the running queries.
	queries, err := db.Query(`select pid, usename, substring(query,1,60), 
	waiting, backend_start from pg_stat_activity order by 5 desc`)
	if err != nil {
		return activeQueryList{}, err
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
		return activeQueryList{}, err
	}

	return ql, nil
}

func getLockList(db *sql.DB) (lockList, error) {
	locks, err := db.Query(`SELECT a.pid, b.relname, a.locktype, a.mode as Lock, a.granted as Lock_holder from pg_locks a, pg_class b where a.relation=b.oid and relname not like 'pg_%' order by 3;`)
	if err != nil {
		return lockList{}, err
	}

	l := lock{}
	ll := lockList{}

	for locks.Next() {
		if err := locks.Scan(&l.Pid, &l.Relation, &l.LockType,
		&l.Lock, &l.LockHolder); err != nil {
			return lockList{}, err
		}

		ll = append(ll, l)
	}

	return ll, nil
}

// A convenience type to feed templates with.
type opstatPkg struct {
	QueryList activeQueryList
	LockList  lockList
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

type lock struct {
	Pid int
	Relation string
	LockType string
	Lock     string
	LockHolder bool
}

type lockList []lock
