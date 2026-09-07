// admin promotes an existing verified account; it cannot bypass email proof.
package main

import (
	"ai-travel/internal/config"
	"ai-travel/internal/store"
	"context"
	"log"
)

func main() {
	url, e := config.Require("DATABASE_URL")
	if e != nil {
		log.Fatal(e)
	}
	email, e := config.Require("ADMIN_EMAIL")
	if e != nil {
		log.Fatal(e)
	}
	s, e := store.Open(context.Background(), url)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if e = s.Migrate(context.Background()); e != nil {
		log.Fatal(e)
	}
	if e = s.PromoteVerifiedUser(context.Background(), email); e != nil {
		log.Fatal(e)
	}
	log.Print("verified account promoted to administrator")
}
