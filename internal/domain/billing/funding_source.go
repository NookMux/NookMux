package billing

import (
	"github.com/NookMux/NookMux/internal/store/user"
)

// FundingSource abstracts a pre-consume / settle / refund lifecycle for a funding source.
// Subscription billing is removed; currently only wallet funding is supported.
type FundingSource interface {
	Source() string
	PreConsume(amount int) error
	Settle(delta int) error
	Refund() error
}

type WalletFunding struct {
	userId   int
	consumed int
}

func (w *WalletFunding) Source() string { return BillingSourceWallet }

func (w *WalletFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	if err := userstore.DecreaseUserQuota(w.userId, amount); err != nil {
		return err
	}
	w.consumed = amount
	return nil
}

func (w *WalletFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return userstore.DecreaseUserQuota(w.userId, delta)
	}
	return userstore.IncreaseUserQuota(w.userId, -delta)
}

func (w *WalletFunding) Refund() error {
	if w.consumed <= 0 {
		return nil
	}
	return userstore.IncreaseUserQuota(w.userId, w.consumed)
}
