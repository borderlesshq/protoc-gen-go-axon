package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/borderlesshq/protoc-gen-go-axon/_labs/srv"
	"github.com/borderlesshq/protoc-gen-go-axon/contracts/accounts"
	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}

	accountsService := srv.NewCustomAccountsService()
	server, err := accounts.RegisterAccountServiceServer(nc, accountsService, accounts.WithPlaygroundEnabled(4030))
	if err != nil {
		log.Fatal(err)
	}

	go server.Serve()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("received signal %v, shutting down", sig)
		if err := server.Shutdown(); err != nil {
			log.Fatal(err)
		}
	}
}
