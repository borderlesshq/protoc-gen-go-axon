package main

import (
	"context"
	"io"
	"log"
	"time"

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

	go func() {
		select {
		case <-time.After(time.Minute * 1):
			if err := stream.CloseSend(); err != nil {
				log.Fatal(err)
			}
		}
	}()
	for {

		out, err := stream.Recv()
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}

		log.Printf("Received wallet update: %v", out)
	}

}
