package financialstatementsspacecoa

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"
	"time"

	"github.com/go-accounting/coa"
	"github.com/go-accounting/deb"
	financialstatements "github.com/go-accounting/financial-statements"
)

type dataSource struct {
	space deb.Space
	coa   *coa.CoaRepository
	coaid *string
}

type transactionMetadata struct {
	Memo    string
	Removes int64
}

var incomeStatementGroups = []string{
	"operating", "deduction", "salesTax", "cost", "nonOperatingTax", "incomeTax", "dividends"}

func NewDataSource(space deb.Space, coa *coa.CoaRepository, coaid *string) (financialstatements.DataSource, error) {
	return &dataSource{space, coa, coaid}, nil
}

func (ds *dataSource) Transactions(accounts []string, from, to time.Time) (chan *financialstatements.Transaction, chan error) {
	ch := make(chan *financialstatements.Transaction)
	errch := make(chan error, 1)
	check := func(err error) bool {
		if err != nil {
			close(ch)
			errch <- err
		}
		return err != nil
	}
	go func() {
		aa, err := ds.debAccounts(accounts)
		if check(err) {
			return
		}
		space, err := ds.space.Slice(
			aa,
			[]deb.DateRange{{deb.DateFromTime(from), deb.DateFromTime(to)}},
			nil)
		if check(err) {
			return
		}
		coaAccounts, err := ds.coa.AllAccounts(*ds.coaid)
		if check(err) {
			return
		}
		accountsIds := make([]string, len(coaAccounts))
		for i, a := range coaAccounts {
			accountsIds[i] = a.Id
		}
		indexes, err := ds.coa.Indexes(*ds.coaid, accountsIds, nil)
		if check(err) {
			return
		}
		sch, serrch := space.Transactions()
		for t := range sch {
			entries := financialstatements.Entries{}
			for k, v := range t.Entries {
				for i := range indexes {
					if k-1 == deb.Account(indexes[i]) {
						entries[coaAccounts[i].Id] += v
						break
					}
				}
			}
			buf := bytes.NewBuffer(t.Metadata)
			dec := gob.NewDecoder(buf)
			var tm transactionMetadata
			err := dec.Decode(&tm)
			if check(err) {
				return
			}
			removes := ""
			if tm.Removes != -1 {
				removes = strconv.Itoa(int(tm.Removes))
			}
			fst := &financialstatements.Transaction{
				Id:      fmt.Sprint(t.Moment),
				Date:    t.Date.ToTime(),
				Entries: entries,
				Memo:    tm.Memo,
				Removes: removes,
				Created: t.Moment.ToTime(),
			}
			ch <- fst
		}
		close(ch)
		errch <- <-serrch
	}()
	return ch, errch
}

func (ds *dataSource) Balances(accounts []string, from, to time.Time) (financialstatements.Entries, error) {
	aa, err := ds.debAccounts(accounts)
	if err != nil {
		return nil, err
	}
	space, err := ds.space.Projection(
		aa,
		[]deb.DateRange{{deb.DateFromTime(from), deb.DateFromTime(to)}},
		nil)
	if err != nil {
		return nil, err
	}
	coaAccounts, err := ds.coa.AllAccounts(*ds.coaid)
	if err != nil {
		return nil, err
	}
	accountsIds := make([]string, len(coaAccounts))
	for i, a := range coaAccounts {
		accountsIds[i] = a.Id
	}
	indexes, err := ds.coa.Indexes(*ds.coaid, accountsIds, nil)
	if err != nil {
		return nil, err
	}
	result := financialstatements.Entries{}
	ch, errch := space.Transactions()
	for t := range ch {
		for k, v := range t.Entries {
			for i := range indexes {
				if k-1 == deb.Account(indexes[i]) {
					result[coaAccounts[i].Id] += v
					break
				}
			}
		}
	}
	return result, <-errch
}

func (ds *dataSource) Accounts() ([]*financialstatements.Account, error) {
	aa, err := ds.coa.AllAccounts(*ds.coaid)
	if err != nil {
		return nil, err
	}
	result := make([]*financialstatements.Account, len(aa))
	for i, a := range aa {
		result[i] = coaAccountToFsAccount(a)
	}
	return result, nil
}

func (ds *dataSource) Account(id string) (*financialstatements.Account, error) {
	a, err := ds.coa.GetAccount(*ds.coaid, id)
	if err != nil {
		return nil, err
	}
	return coaAccountToFsAccount(a), nil
}

func (ds *dataSource) IsParent(parent, child string) bool {
	c, err := ds.coa.GetAccount(*ds.coaid, child)
	if err != nil {
		return false
	}
	return c.Parent == parent
}

func (ds *dataSource) debAccounts(ids []string) ([]deb.Account, error) {
	idxs, err := ds.coa.Indexes(*ds.coaid, ids, nil)
	if err != nil {
		return nil, err
	}
	result := make([]deb.Account, len(idxs))
	for i, _ := range idxs {
		result[i] = deb.Account(idxs[i] + 1)
	}
	return result, nil
}

func incomeStatementGroup(a *coa.Account) string {
	for _, g := range incomeStatementGroups {
		if a.Tags.Contains(g) {
			return g
		}
	}
	return ""
}

func coaAccountToFsAccount(a *coa.Account) *financialstatements.Account {
	return &financialstatements.Account{
		Id:                   a.Id,
		Number:               a.Number,
		Name:                 a.Name,
		Summary:              a.Tags.Contains("summary"),
		IncreaseOnDebit:      a.Tags.Contains("increaseOnDebit"),
		BalanceSheet:         a.Tags.Contains("balanceSheet"),
		IncomeStatement:      a.Tags.Contains("incomeStatement"),
		IncomeStatementGroup: incomeStatementGroup(a),
	}
}
