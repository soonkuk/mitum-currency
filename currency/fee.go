package currency

import (
	"time"

	"golang.org/x/xerrors"

	"github.com/spikeekips/mitum/base"
	"github.com/spikeekips/mitum/base/operation"
	"github.com/spikeekips/mitum/base/state"
	"github.com/spikeekips/mitum/util"
	"github.com/spikeekips/mitum/util/hint"
	"github.com/spikeekips/mitum/util/isvalid"
	"github.com/spikeekips/mitum/util/valuehash"
)

var (
	FeeOperationFactType = hint.MustNewType(0xa0, 0x12, "mitum-currency-fee-operation-fact")
	FeeOperationFactHint = hint.MustHint(FeeOperationFactType, "0.0.1")
	FeeOperationType     = hint.MustNewType(0xa0, 0x13, "mitum-currency-fee-operation")
	FeeOperationHint     = hint.MustHint(FeeOperationType, "0.0.1")
)

type FeeOperationFact struct {
	h       valuehash.Hash
	token   []byte
	amounts []Amount
}

func NewFeeOperationFact(height base.Height, ams map[CurrencyID]Big) FeeOperationFact {
	amounts := make([]Amount, len(ams))
	var i int
	for cid := range ams {
		amounts[i] = NewAmount(ams[cid], cid)
		i++
	}

	// TODO replace random bytes with height
	fact := FeeOperationFact{
		token:   height.Bytes(), // for unique token
		amounts: amounts,
	}
	fact.h = valuehash.NewSHA256(fact.Bytes())

	return fact
}

func (fact FeeOperationFact) Hint() hint.Hint {
	return FeeOperationFactHint
}

func (fact FeeOperationFact) Hash() valuehash.Hash {
	return fact.h
}

func (fact FeeOperationFact) Bytes() []byte {
	bs := make([][]byte, len(fact.amounts)+1)
	bs[0] = fact.token

	for i := range fact.amounts {
		bs[i+1] = fact.amounts[i].Bytes()
	}

	return util.ConcatBytesSlice(bs...)
}

func (fact FeeOperationFact) IsValid([]byte) error {
	if len(fact.token) < 1 {
		return xerrors.Errorf("empty token for FeeOperationFact")
	}

	if err := fact.h.IsValid(nil); err != nil {
		return err
	}

	for i := range fact.amounts {
		if err := fact.amounts[i].IsValid(nil); err != nil {
			return err
		}
	}

	return nil
}

func (fact FeeOperationFact) Token() []byte {
	return fact.token
}

func (fact FeeOperationFact) Amounts() []Amount {
	return fact.amounts
}

type FeeOperation struct {
	fact FeeOperationFact
	h    valuehash.Hash
}

func NewFeeOperation(fact FeeOperationFact) FeeOperation {
	op := FeeOperation{fact: fact}
	op.h = op.GenerateHash()

	return op
}

func (op FeeOperation) Hint() hint.Hint {
	return FeeOperationHint
}

func (op FeeOperation) Fact() base.Fact {
	return op.fact
}

func (op FeeOperation) Hash() valuehash.Hash {
	return op.h
}

func (op FeeOperation) Signs() []operation.FactSign {
	return nil
}

func (op FeeOperation) IsValid([]byte) error {
	if err := op.Hint().IsValid(nil); err != nil {
		return err
	}

	if l := len(op.fact.Token()); l < 1 {
		return isvalid.InvalidError.Errorf("FeeOperation has empty token")
	} else if l > operation.MaxTokenSize {
		return isvalid.InvalidError.Errorf("FeeOperation token size too large: %d > %d", l, operation.MaxTokenSize)
	}

	if err := op.Fact().IsValid(nil); err != nil {
		return err
	}

	if !op.Hash().Equal(op.GenerateHash()) {
		return isvalid.InvalidError.Errorf("wrong FeeOperation hash")
	}

	return nil
}

func (op FeeOperation) GenerateHash() valuehash.Hash {
	return valuehash.NewSHA256(op.Fact().Hash().Bytes())
}

func (op FeeOperation) AddFactSigns(...operation.FactSign) (operation.FactSignUpdater, error) {
	return nil, nil
}

func (op FeeOperation) LastSignedAt() time.Time {
	return time.Time{}
}

func (op FeeOperation) Process(
	func(key string) (state.State, bool, error),
	func(valuehash.Hash, ...state.State) error,
) error {
	return nil
}

type FeeOperationProcessor struct {
	FeeOperation
	cp *CurrencyPool
}

func NewFeeOperationProcessor(cp *CurrencyPool, op FeeOperation) state.Processor {
	return &FeeOperationProcessor{
		cp:           cp,
		FeeOperation: op,
	}
}

func (opp *FeeOperationProcessor) Process(
	getState func(key string) (state.State, bool, error),
	setState func(valuehash.Hash, ...state.State) error,
) error {
	fact := opp.Fact().(FeeOperationFact)

	sts := make([]state.State, len(fact.amounts))
	for i := range fact.amounts {
		am := fact.amounts[i]
		var feeer Feeer
		if j, found := opp.cp.Feeer(am.Currency()); !found {
			return xerrors.Errorf("unknown currency id, %q found for FeeOperation", am.Currency())
		} else {
			feeer = j
		}

		if feeer.Receiver() == nil {
			continue
		}

		if err := checkExistsState(StateKeyAccount(feeer.Receiver()), getState); err != nil {
			return err
		} else if st, _, err := getState(StateKeyBalance(feeer.Receiver(), am.Currency())); err != nil {
			return err
		} else {
			rb := NewAmountState(st, am.Currency())

			sts[i] = rb.Add(am.Big())
		}
	}

	return setState(fact.Hash(), sts...)
}
