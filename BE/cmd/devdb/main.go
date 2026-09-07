// devdb starts a real PostgreSQL instance in a private project directory for local verification.
package main

import (
	"fmt"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	root, e := filepath.Abs(".local/postgres")
	if e != nil {
		log.Fatal(e)
	}
	p := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().Port(15432).Database("travel").Username("travel").Password("local-travel-only").Version(embeddedpostgres.V16).RuntimePath(root + "/runtime").DataPath(root + "/data").BinariesPath(root + "/bin").CachePath(root + "/cache").BinaryRepositoryURL("https://repo.maven.apache.org/maven2").Logger(os.Stdout))
	if e = p.Start(); e != nil {
		log.Fatal(e)
	}
	fmt.Println("Local PostgreSQL listening on 127.0.0.1:15432 (database travel)")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	if e = p.Stop(); e != nil {
		log.Print(e)
	}
}
