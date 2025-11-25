package srv

import (
	"context"
	"fmt"
	"time"

	"github.com/borderlesshq/protoc-gen-go-axon/contracts/accounts"
)

type CustomAccountsService struct {
}

func (c CustomAccountsService) StreamWalletUpdatesFromServer(request *accounts.GetByIdRequest, server accounts.AccountService_StreamWalletUpdatesFromServerServer) error {
	list := FakeWallets(2000)
	for _, w := range list {
		if err := server.Send(w); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func (c CustomAccountsService) StreamWalletUpdatesFromClient(server accounts.AccountService_StreamWalletUpdatesFromClientServer) error {
	for {
		in, err := server.Recv()
		if err != nil {
			return err
		}

		fmt.Println(in.Id)
	}
}

func (c CustomAccountsService) CreateWallet(ctx context.Context, req *accounts.CreateWalletRequest) (*accounts.Wallet, error) {
	wallet := &accounts.Wallet{
		Id:       fmt.Sprintf("wlt_%d", time.Now().Unix()),
		Name:     req.Name,
		Currency: req.Currency,
		Status:   "ACTIVE",
	}
	return wallet, nil
}

func (c CustomAccountsService) GetWallet(ctx context.Context, request *accounts.GetByIdRequest) (*accounts.Wallet, error) {
	return FakeWalletWithID(request.GetId()), nil
}

func (c CustomAccountsService) ListWallets(ctx context.Context, request *accounts.ListWalletsRequest) (*accounts.ListWalletsResponse, error) {
	list := FakeWallets(10)
	return &accounts.ListWalletsResponse{
		Data:           list,
		NextPageCursor: "lorem",
		PrevPageCursor: "ispum",
	}, nil
}

func (c CustomAccountsService) SetWalletPin(ctx context.Context, request *accounts.SetWalletPinRequest) (*accounts.RemarksResponse, error) {
	return &accounts.RemarksResponse{
		Remarks: "Wallet PIN set successfully",
	}, nil
}

func (c CustomAccountsService) ConfirmSetWalletPin(ctx context.Context, request *accounts.ConfirmSetWalletPinRequest) (*accounts.RemarksResponse, error) {
	return &accounts.RemarksResponse{
		Remarks: "Wallet PIN set successfully",
	}, nil
}

func (c CustomAccountsService) CreateStaticAccount(ctx context.Context, request *accounts.CreateStaticAccountRequest) (*accounts.Account, error) {
	//TODO implement me
	panic("implement me")
}

func (c CustomAccountsService) CreateVirtualAccount(ctx context.Context, request *accounts.CreateVirtualAccountRequest) (*accounts.Account, error) {
	//TODO implement me
	panic("implement me")
}

func (c CustomAccountsService) GetAccount(ctx context.Context, request *accounts.GetByIdRequest) (*accounts.Account, error) {
	//TODO implement me
	panic("implement me")
}

func (c CustomAccountsService) ListAccounts(ctx context.Context, request *accounts.ListAccountsRequest) (*accounts.ListAccountsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c CustomAccountsService) StreamWalletUpdates(server accounts.AccountService_StreamWalletUpdatesServer) error {
	list := FakeWallets(2000)
	for _, w := range list {
		go func() {
			out, err := server.Recv()
			if err != nil {
				fmt.Println("error receiving from client: ", err)
				return
			}
			fmt.Println("received from client in bidi: ", out)
		}()
		if err := server.Send(w); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func NewCustomAccountsService() accounts.AccountServiceServer {
	return &CustomAccountsService{}
}

// FakeWallet returns a synthetic *accounts.Wallet. Optional mutators can modify fields.
func FakeWallet(opts ...func(*accounts.Wallet)) *accounts.Wallet {
	w := &accounts.Wallet{
		Id:       fmt.Sprintf("wlt_%d", time.Now().UnixNano()),
		Name:     "Fake Wallet",
		Currency: "USD",
		Status:   "ACTIVE",
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// FakeWalletWithID returns a fake wallet with the provided id.
func FakeWalletWithID(id string) *accounts.Wallet {
	return FakeWallet(func(w *accounts.Wallet) {
		w.Id = id
	})
}

// FakeWallets returns n fake wallets. Each wallet Name includes an index.
func FakeWallets(n int) []*accounts.Wallet {
	if n <= 0 {
		return []*accounts.Wallet{}
	}
	currencies := []string{"USD", "EUR", "GBP", "NGN", "JPY"}
	statuses := []string{"ACTIVE", "INACTIVE", "SUSPENDED"}
	res := make([]*accounts.Wallet, 0, n)
	for i := 0; i < n; i++ {
		idx := i + 1
		seed := time.Now().UnixNano() + int64(i)
		currency := currencies[int(seed)%len(currencies)]
		status := statuses[int((seed/int64(len(currencies)))%int64(len(statuses)))]
		id := fmt.Sprintf("wlt_%d_%d", seed, i)
		res = append(res, FakeWallet(func(w *accounts.Wallet) {
			w.Id = id
			w.Name = fmt.Sprintf("Fake Wallet %d", idx)
			w.Currency = currency
			w.Status = status
		}))
	}
	return res
}
