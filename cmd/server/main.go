package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"

	"github.com/SP203/Token-Transfer-API/graph"
	"github.com/SP203/Token-Transfer-API/graph/generated"
	"github.com/SP203/Token-Transfer-API/internal/db"
)

func main() {
	dsn := os.Getenv("BTP_DB_DSN")
	if dsn == "" {
		dsn = "postgres://btp:btp@localhost:5432/btp"
	}

	sqlDB, err := db.Open(db.Config{DSN: dsn})
	if err != nil {
		log.Fatal("cannot open db: ", err)
	}
	db.MustPing(context.Background(), sqlDB)

	if err := db.Init(context.Background(), sqlDB); err != nil {
		log.Fatal("db init failed: ", err)
	}

	repo := db.NewRepo(sqlDB)
	res := &graph.Resolver{Repo: repo}

	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: res}))
	http.Handle("/", playground.Handler("GraphQL Playground", "/query"))
	http.Handle("/query", srv)

	log.Println("🚀 Server running on http://localhost:8080/")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
