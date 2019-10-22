package actions

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	protocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/db2/core"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/xdr"
)

var (
	trustLineIssuer = "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"
	accountOne      = "GABGMPEKKDWR2WFH5AJOZV5PDKLJEHGCR3Q24ALETWR5H3A7GI3YTS7V"
	accountTwo      = "GADTXHUTHIAESMMQ2ZWSTIIGBZRLHUCBLCHPLLUEIAWDEFRDC4SYDKOZ"
	accountThree    = "GDP347UYM2ZKE6ED6T5OM3BQ5IAS76NKRVEUPNB5PCQ26Z5D7Q7PJOMI"
	signer          = "GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU"
	usd             = xdr.MustNewCreditAsset("USD", trustLineIssuer)
	euro            = xdr.MustNewCreditAsset("EUR", trustLineIssuer)

	account1 = xdr.AccountEntry{
		AccountId:     xdr.MustAddress(accountOne),
		Balance:       20000,
		SeqNum:        223456789,
		NumSubEntries: 10,
		Flags:         1,
		HomeDomain:    "stellar.org",
		Thresholds:    xdr.Thresholds{1, 2, 3, 4},
		Ext: xdr.AccountEntryExt{
			V: 1,
			V1: &xdr.AccountEntryV1{
				Liabilities: xdr.Liabilities{
					Buying:  3,
					Selling: 4,
				},
			},
		},
	}

	account2 = xdr.AccountEntry{
		AccountId:     xdr.MustAddress(accountTwo),
		Balance:       50000,
		SeqNum:        648736,
		NumSubEntries: 10,
		Flags:         2,
		HomeDomain:    "meridian.stellar.org",
		Thresholds:    xdr.Thresholds{5, 6, 7, 8},
		Ext: xdr.AccountEntryExt{
			V: 1,
			V1: &xdr.AccountEntryV1{
				Liabilities: xdr.Liabilities{
					Buying:  30,
					Selling: 40,
				},
			},
		},
	}
	eurTrustLine = xdr.TrustLineEntry{
		AccountId: xdr.MustAddress(accountOne),
		Asset:     euro,
		Balance:   20000,
		Limit:     223456789,
		Flags:     1,
		Ext: xdr.TrustLineEntryExt{
			V: 1,
			V1: &xdr.TrustLineEntryV1{
				Liabilities: xdr.Liabilities{
					Buying:  3,
					Selling: 4,
				},
			},
		},
	}

	usdTrustLine = xdr.TrustLineEntry{
		AccountId: xdr.MustAddress(accountTwo),
		Asset:     usd,
		Balance:   10000,
		Limit:     123456789,
		Flags:     0,
		Ext: xdr.TrustLineEntryExt{
			V: 1,
			V1: &xdr.TrustLineEntryV1{
				Liabilities: xdr.Liabilities{
					Buying:  1,
					Selling: 2,
				},
			},
		},
	}

	data1 = xdr.DataEntry{
		AccountId: xdr.MustAddress(accountOne),
		DataName:  "test data",
		// This also tests if base64 encoding is working as 0 is invalid UTF-8 byte
		DataValue: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	data2 = xdr.DataEntry{
		AccountId: xdr.MustAddress(accountTwo),
		DataName:  "test data2",
		DataValue: []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
	}
)

func TestAccountInfo(t *testing.T) {
	tt := test.Start(t).Scenario("allow_trust")
	defer tt.Finish()

	account, err := AccountInfo(tt.Ctx, &core.Q{tt.CoreSession()}, signer)
	tt.Assert.NoError(err)

	tt.Assert.Equal("8589934593", account.Sequence)
	tt.Assert.NotEqual(0, account.LastModifiedLedger)

	for _, balance := range account.Balances {
		if balance.Type == "native" {
			tt.Assert.Equal(uint32(0), balance.LastModifiedLedger)
		} else {
			tt.Assert.NotEqual(uint32(0), balance.LastModifiedLedger)
		}
	}
}
func TestGetAccountsHandlerPageNoResults(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &history.Q{tt.HorizonSession()}
	handler := &GetAccountsHandler{HistoryQ: q}
	records, err := handler.GetResourcePage(
		httptest.NewRecorder(),
		makeRequest(
			t,
			map[string]string{
				"signer": signer,
			},
			map[string]string{},
			q.Session,
		),
	)
	tt.Assert.NoError(err)
	tt.Assert.Len(records, 0)
}

func TestGetAccountsHandlerPageResultsBySigner(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &history.Q{tt.HorizonSession()}
	handler := &GetAccountsHandler{HistoryQ: q}

	rows := accountSigners()

	for _, row := range rows {
		q.CreateAccountSigner(row.Account, row.Signer, row.Weight)
	}

	records, err := handler.GetResourcePage(
		httptest.NewRecorder(),
		makeRequest(
			t,
			map[string]string{
				"signer": signer,
			},
			map[string]string{},
			q.Session,
		),
	)

	tt.Assert.NoError(err)
	tt.Assert.Equal(3, len(records))

	for i, row := range rows {
		result := records[i].(protocol.AccountSigner)
		tt.Assert.Equal(row.Account, result.AccountID)
		tt.Assert.Equal(row.Signer, result.Signer.Key)
		tt.Assert.Equal(row.Weight, result.Signer.Weight)
	}

	records, err = handler.GetResourcePage(
		httptest.NewRecorder(),
		makeRequest(
			t,
			map[string]string{
				"signer": "GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU",
				"cursor": "GABGMPEKKDWR2WFH5AJOZV5PDKLJEHGCR3Q24ALETWR5H3A7GI3YTS7V",
			},
			map[string]string{},
			q.Session,
		),
	)

	tt.Assert.NoError(err)
	tt.Assert.Equal(2, len(records))

	for i, row := range rows[1:] {
		result := records[i].(protocol.AccountSigner)
		tt.Assert.Equal(row.Account, result.AccountID)
		tt.Assert.Equal(row.Signer, result.Signer.Key)
		tt.Assert.Equal(row.Weight, result.Signer.Weight)
	}
}

func TestGetAccountsHandlerPageResultsByAsset(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &history.Q{tt.HorizonSession()}
	handler := &GetAccountsHandler{HistoryQ: q}

	_, err := q.InsertAccount(account1, 1234)
	tt.Assert.NoError(err)
	_, err = q.InsertAccount(account2, 1234)
	tt.Assert.NoError(err)

	rows := accountSigners()

	for _, row := range rows {
		_, err = q.CreateAccountSigner(row.Account, row.Signer, row.Weight)
		tt.Assert.NoError(err)
	}

	_, err = q.InsertAccountData(data1, 1234)
	assert.NoError(t, err)
	_, err = q.InsertAccountData(data2, 1234)
	assert.NoError(t, err)

	var assetType, code, issuer string
	usd.MustExtract(&assetType, &code, &issuer)
	params := map[string]string{
		"asset_issuer": issuer,
		"asset_code":   code,
		"asset_type":   assetType,
	}

	records, err := handler.GetResourcePage(
		httptest.NewRecorder(),
		makeRequest(
			t,
			params,
			map[string]string{},
			q.Session,
		),
	)

	tt.Assert.NoError(err)
	tt.Assert.Equal(0, len(records))

	_, err = q.InsertTrustLine(eurTrustLine, 1234)
	assert.NoError(t, err)
	_, err = q.InsertTrustLine(usdTrustLine, 1235)
	assert.NoError(t, err)

	records, err = handler.GetResourcePage(
		httptest.NewRecorder(),
		makeRequest(
			t,
			params,
			map[string]string{},
			q.Session,
		),
	)

	tt.Assert.NoError(err)
	tt.Assert.Equal(1, len(records))
	result := records[0].(protocol.Account)
	tt.Assert.Equal(accountTwo, result.AccountID)
	tt.Assert.Len(result.Balances, 2)
	tt.Assert.Len(result.Signers, 2)

	_, ok := result.Data[string(data2.DataName)]
	tt.Assert.True(ok)

}

func accountSigners() []history.AccountSigner {
	return []history.AccountSigner{
		history.AccountSigner{
			Account: accountOne,
			Signer:  signer,
			Weight:  1,
		},
		history.AccountSigner{
			Account: accountTwo,
			Signer:  signer,
			Weight:  2,
		},
		history.AccountSigner{
			Account: accountThree,
			Signer:  signer,
			Weight:  3,
		},
	}
}
