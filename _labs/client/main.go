package main

import (
	"context"
	"log"

	"github.com/borderlesshq/protoc-gen-go-axon/contracts/accounts"
	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}

	accountsClient := accounts.NewAccountServiceClient(nc)

	stream, err := accountsClient.StreamWalletUpdates(context.Background())

	for {

		out, err := stream.Recv()
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Received wallet update: %v", out)
	}

}
