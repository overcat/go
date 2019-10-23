package actions

import (
	"context"
	"net/http"

	protocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/db2/core"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/resourceadapter"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/render/hal"
)

// AccountInfo returns the information about an account identified by addr.
func AccountInfo(ctx context.Context, cq *core.Q, addr string) (*protocol.Account, error) {
	var (
		coreRecord     core.Account
		coreData       []core.AccountData
		coreSigners    []core.Signer
		coreTrustlines []core.Trustline
		resource       protocol.Account
	)

	err := cq.AccountByAddress(&coreRecord, addr)
	if err != nil {
		return nil, errors.Wrap(err, "getting core account record")
	}

	err = cq.AllDataByAddress(&coreData, addr)
	if err != nil {
		return nil, errors.Wrap(err, "getting core account data")
	}

	err = cq.SignersByAddress(&coreSigners, addr)
	if err != nil {
		return nil, errors.Wrap(err, "getting core signer")
	}

	err = cq.TrustlinesByAddress(&coreTrustlines, addr)
	if err != nil {
		return nil, errors.Wrap(err, "getting core trustline")
	}

	err = resourceadapter.PopulateAccount(
		ctx,
		&resource,
		coreRecord,
		coreData,
		coreSigners,
		coreTrustlines,
	)

	return &resource, errors.Wrap(err, "populating account")
}

// GetAccountsHandler is the action handler for the /accounts endpoint
type GetAccountsHandler struct {
	HistoryQ *history.Q
}

// GetResourcePage returns a page containing the account records that have
// `signer` as a signer. This doesn't return full account details resource
// because of the limitations of existing ingestion architecture. In a future,
// when the new ingestion system is fully integrated, this endpoint can be used
// to find accounts for signer but also accounts for assets, home domain,
// inflation_dest etc.
func (handler GetAccountsHandler) GetResourcePage(
	w HeaderWriter,
	r *http.Request,
) ([]hal.Pageable, error) {
	ctx := r.Context()
	pq, err := GetPageQuery(r, DisableCursorValidation)
	if err != nil {
		return nil, err
	}

	rawSigner, err := GetString(r, "signer")
	if err != nil {
		return nil, err
	}
	var accounts []hal.Pageable

	historyQ, err := historyQFromRequest(r)
	if err != nil {
		return nil, err
	}

	if len(rawSigner) > 0 {

		signer, err := GetAccountID(r, "signer")
		if err != nil {
			return nil, err
		}
		records, err := historyQ.AccountsForSigner(signer.Address(), pq)
		if err != nil {
			return nil, errors.Wrap(err, "loading account records")
		}

		for _, record := range records {
			var res protocol.AccountSigner
			resourceadapter.PopulateAccountSigner(ctx, &res, record)
			accounts = append(accounts, res)
		}
	} else {
		asset, err := GetAsset(r, "")
		if err != nil {
			return nil, err
		}

		records, err := historyQ.AccountsForAsset(asset, pq)
		if err != nil {
			return nil, errors.Wrap(err, "loading account records")
		}

		if len(records) == 0 {
			// early return
			return accounts, nil
		}

		accountIDs := make([]string, 0, len(records))
		for _, record := range records {
			accountIDs = append(accountIDs, record.AccountID)
		}

		signers, err := handler.loadSigners(handler.HistoryQ, accountIDs)
		if err != nil {
			return nil, err
		}

		trustlines, err := handler.loadTrustlines(handler.HistoryQ, accountIDs)
		if err != nil {
			return nil, err
		}

		data, err := handler.loadData(handler.HistoryQ, accountIDs)
		if err != nil {
			return nil, err
		}

		for _, record := range records {
			var res protocol.Account
			s, ok := signers[record.AccountID]
			if !ok {
				s = []history.AccountSigner{}
			}

			t, ok := trustlines[record.AccountID]
			if !ok {
				t = []history.TrustLine{}
			}

			d, ok := data[record.AccountID]
			if !ok {
				d = []history.Data{}
			}

			resourceadapter.PopulateAccountEntry(ctx, &res, record, d, s, t)

			accounts = append(accounts, res)
		}
	}

	return accounts, nil
}

func (handler GetAccountsHandler) loadData(historyQ *history.Q, accounts []string) (map[string][]history.Data, error) {
	data := make(map[string][]history.Data)

	records, err := historyQ.GetAccountDataByAccountsID(accounts)
	if err != nil {
		return data, err
	}

	for _, record := range records {
		data[record.AccountID] = append(data[record.AccountID], record)
	}

	return data, nil
}

func (handler GetAccountsHandler) loadTrustlines(historyQ *history.Q, accounts []string) (map[string][]history.TrustLine, error) {
	trustLines := make(map[string][]history.TrustLine)

	records, err := historyQ.GetTrustLinesByAccountsID(accounts)
	if err != nil {
		return trustLines, err
	}

	for _, record := range records {
		trustLines[record.AccountID] = append(trustLines[record.AccountID], record)
	}

	return trustLines, nil
}

func (handler GetAccountsHandler) loadSigners(historyQ *history.Q, accounts []string) (map[string][]history.AccountSigner, error) {
	signers := make(map[string][]history.AccountSigner)

	records, err := historyQ.SignersForAccounts(accounts)
	if err != nil {
		return signers, err
	}

	for _, record := range records {
		signers[record.Account] = append(signers[record.Account], record)
	}

	return signers, nil
}
