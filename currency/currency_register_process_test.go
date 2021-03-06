package currency

import (
	"testing"

	"github.com/spikeekips/mitum/base"
	"github.com/spikeekips/mitum/base/key"
	"github.com/spikeekips/mitum/base/operation"
	"github.com/spikeekips/mitum/base/state"
	"github.com/spikeekips/mitum/util"
	"github.com/stretchr/testify/suite"
	"golang.org/x/xerrors"
)

type testCurrencyRegisterOperations struct {
	baseTestOperationProcessor
	cid CurrencyID
}

func (t *testCurrencyRegisterOperations) SetupSuite() {
	t.cid = CurrencyID("SHOWME")
}

func (t *testCurrencyRegisterOperations) newOperation(keys []key.Privatekey, item CurrencyDesign) CurrencyRegister {
	token := util.UUID().Bytes()
	fact := NewCurrencyRegisterFact(token, item)

	var fs []operation.FactSign
	for _, pk := range keys {
		sig, err := operation.NewFactSignature(pk, fact, nil)
		t.NoError(err)

		fs = append(fs, operation.NewBaseFactSign(pk.Publickey(), sig))
	}

	tf, err := NewCurrencyRegister(fact, fs, "")
	t.NoError(err)

	t.NoError(tf.IsValid(nil))

	return tf
}

func (t *testCurrencyRegisterOperations) processor(n int) ([]key.Privatekey, *OperationProcessor) {
	privs := make([]key.Privatekey, n)
	for i := 0; i < n; i++ {
		privs[i] = key.MustNewBTCPrivatekey()
	}

	pubs := make([]key.Publickey, len(privs))
	for i := range privs {
		pubs[i] = privs[i].Publickey()
	}
	threshold, err := base.NewThreshold(uint(len(privs)), 100)
	t.NoError(err)

	opr := NewOperationProcessor(nil)
	_, err = opr.SetProcessor(CurrencyRegister{}, NewCurrencyRegisterProcessor(nil, pubs, threshold))
	t.NoError(err)

	return privs, opr
}

func (t *testCurrencyRegisterOperations) currencyDesign(big Big, cid CurrencyID, ga base.Address) CurrencyDesign {
	return NewCurrencyDesign(NewAmount(big, cid), ga, NewCurrencyPolicy(ZeroBig, NewNilFeeer()))
}

func (t *testCurrencyRegisterOperations) TestGenesisAddressNotExist() {
	privs, copr := t.processor(3)

	ga, _ := t.newAccount(false, nil)

	item := t.currencyDesign(NewBig(33), t.cid, ga.Address)
	tf := t.newOperation(privs, item)

	pool, _ := t.statepool()
	opr := copr.New(pool)

	err := opr.Process(tf)

	var oper operation.ReasonError
	t.True(xerrors.As(err, &oper))
	t.Contains(err.Error(), "does not exist")
}

func (t *testCurrencyRegisterOperations) TestSameCurrencyID() {
	privs, copr := t.processor(3)

	cid := CurrencyID("FINDME")

	var sts []state.State
	var op0, op1 CurrencyRegister
	{
		ga, s := t.newAccount(true, []Amount{NewAmount(NewBig(10), t.cid)})
		item := t.currencyDesign(NewBig(33), cid, ga.Address)
		op0 = t.newOperation(privs, item)

		sts = append(sts, s...)
	}

	{
		ga, s := t.newAccount(true, []Amount{NewAmount(NewBig(10), t.cid)})
		item := t.currencyDesign(NewBig(44), cid, ga.Address)
		op1 = t.newOperation(privs, item)

		sts = append(sts, s...)
	}

	pool, _ := t.statepool(sts)
	opr := copr.New(pool)

	t.NoError(opr.Process(op0))

	err := opr.Process(op1)
	t.Contains(err.Error(), "duplicated currency id")
}

func (t *testCurrencyRegisterOperations) TestEmptyPubs() {
	var sts []state.State

	ga, s := t.newAccount(true, []Amount{NewAmount(NewBig(10), t.cid)})

	cid := CurrencyID("FINDME")
	item := t.currencyDesign(NewBig(33), cid, ga.Address)
	op := t.newOperation(ga.Privs(), item)

	sts = append(sts, s...)

	pool, _ := t.statepool(sts)

	copr := NewOperationProcessor(nil)
	_, err := copr.SetProcessor(CurrencyRegister{}, func(op state.Processor) (state.Processor, error) {
		if i, ok := op.(CurrencyRegister); !ok {
			return nil, xerrors.Errorf("not CurrencyRegister, %T", op)
		} else {
			return &CurrencyRegisterProcessor{
				CurrencyRegister: i,
			}, nil
		}
	})
	t.NoError(err)

	opr := copr.New(pool)

	err = opr.Process(op)
	t.Contains(err.Error(), "empty publickeys")
}

func (t *testCurrencyRegisterOperations) TestNotEnoughSigns() {
	var sts []state.State

	privs, copr := t.processor(3)

	ga, s := t.newAccount(true, []Amount{NewAmount(NewBig(10), t.cid)})

	cid := CurrencyID("FINDME")
	item := t.currencyDesign(NewBig(33), cid, ga.Address)
	op := t.newOperation(privs[:2], item)

	sts = append(sts, s...)

	pool, _ := t.statepool(sts)

	opr := copr.New(pool)

	err := opr.Process(op)
	t.Contains(err.Error(), "not enough suffrage signs")
}

func (t *testCurrencyRegisterOperations) TestNew() {
	var sts []state.State

	privs, copr := t.processor(3)

	ga, s := t.newAccount(true, []Amount{NewAmount(NewBig(10), t.cid)})

	cid := CurrencyID("FINDME")
	item := t.currencyDesign(NewBig(33), cid, ga.Address)
	op := t.newOperation(privs, item)

	sts = append(sts, s...)

	pool, _ := t.statepool(sts)

	opr := copr.New(pool)

	t.NoError(opr.Process(op))

	var gast, gbst state.State
	for _, st := range pool.Updates() {
		switch st.Key() {
		case StateKeyBalance(ga.Address, cid):
			gast = st.GetState()
		case StateKeyCurrencyDesign(cid):
			gbst = st.GetState()
		}
	}

	uga, err := StateBalanceValue(gast)
	t.NoError(err)
	t.True(uga.Big().Equal(item.Big()))
	t.Equal(uga.Currency(), item.Currency())

	ugb, err := StateCurrencyDesignValue(gbst)
	t.NoError(err)
	t.compareCurrencyDesign(ugb, item)
}

func TestCurrencyRegisterOperations(t *testing.T) {
	suite.Run(t, new(testCurrencyRegisterOperations))
}
