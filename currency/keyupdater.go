package currency

import (
	"golang.org/x/xerrors"

	"github.com/spikeekips/mitum/base"
	"github.com/spikeekips/mitum/base/operation"
	"github.com/spikeekips/mitum/util"
	"github.com/spikeekips/mitum/util/hint"
	"github.com/spikeekips/mitum/util/isvalid"
	"github.com/spikeekips/mitum/util/valuehash"
)

var (
	KeyUpdaterFactType = hint.MustNewType(0xa0, 0x09, "mitum-currency-keyupdater-operation-fact")
	KeyUpdaterFactHint = hint.MustHint(KeyUpdaterFactType, "0.0.1")
	KeyUpdaterType     = hint.MustNewType(0xa0, 0x10, "mitum-currency-keyupdater-operation")
	KeyUpdaterHint     = hint.MustHint(KeyUpdaterType, "0.0.1")
)

type KeyUpdaterFact struct {
	h        valuehash.Hash
	token    []byte
	target   base.Address
	keys     Keys
	currency CurrencyID
}

func NewKeyUpdaterFact(token []byte, target base.Address, keys Keys, currency CurrencyID) KeyUpdaterFact {
	fact := KeyUpdaterFact{
		token:    token,
		target:   target,
		keys:     keys,
		currency: currency,
	}
	fact.h = fact.GenerateHash()

	return fact
}

func (fact KeyUpdaterFact) Hint() hint.Hint {
	return KeyUpdaterFactHint
}

func (fact KeyUpdaterFact) Hash() valuehash.Hash {
	return fact.h
}

func (fact KeyUpdaterFact) GenerateHash() valuehash.Hash {
	return valuehash.NewSHA256(fact.Bytes())
}

func (fact KeyUpdaterFact) Bytes() []byte {
	return util.ConcatBytesSlice(
		fact.token,
		fact.target.Bytes(),
		fact.keys.Bytes(),
		fact.currency.Bytes(),
	)
}

func (fact KeyUpdaterFact) IsValid([]byte) error {
	if len(fact.token) < 1 {
		return xerrors.Errorf("empty token for KeyUpdaterFact")
	}

	if err := isvalid.Check([]isvalid.IsValider{
		fact.h,
		fact.target,
		fact.keys,
		fact.currency,
	}, nil, false); err != nil {
		return err
	}

	if !fact.h.Equal(fact.GenerateHash()) {
		return isvalid.InvalidError.Errorf("wrong Fact hash")
	}

	return nil
}

func (fact KeyUpdaterFact) Token() []byte {
	return fact.token
}

func (fact KeyUpdaterFact) Target() base.Address {
	return fact.target
}

func (fact KeyUpdaterFact) Keys() Keys {
	return fact.keys
}

func (fact KeyUpdaterFact) Currency() CurrencyID {
	return fact.currency
}

func (fact KeyUpdaterFact) Addresses() ([]base.Address, error) {
	return []base.Address{fact.target}, nil
}

type KeyUpdater struct {
	operation.BaseOperation
	Memo string
}

func NewKeyUpdater(fact KeyUpdaterFact, fs []operation.FactSign, memo string) (KeyUpdater, error) {
	if bo, err := operation.NewBaseOperationFromFact(KeyUpdaterHint, fact, fs); err != nil {
		return KeyUpdater{}, err
	} else {
		op := KeyUpdater{BaseOperation: bo, Memo: memo}

		op.BaseOperation = bo.SetHash(op.GenerateHash())

		return op, nil
	}
}

func (op KeyUpdater) Hint() hint.Hint {
	return KeyUpdaterHint
}

func (op KeyUpdater) IsValid(networkID []byte) error {
	if err := IsValidMemo(op.Memo); err != nil {
		return err
	}

	return operation.IsValidOperation(op, networkID)
}

func (op KeyUpdater) GenerateHash() valuehash.Hash {
	bs := make([][]byte, len(op.Signs())+1)
	for i := range op.Signs() {
		bs[i] = op.Signs()[i].Bytes()
	}

	bs[len(bs)-1] = []byte(op.Memo)

	e := util.ConcatBytesSlice(op.Fact().Hash().Bytes(), util.ConcatBytesSlice(bs...))

	return valuehash.NewSHA256(e)
}

func (op KeyUpdater) AddFactSigns(fs ...operation.FactSign) (operation.FactSignUpdater, error) {
	if o, err := op.BaseOperation.AddFactSigns(fs...); err != nil {
		return nil, err
	} else {
		op.BaseOperation = o.(operation.BaseOperation)
	}

	op.BaseOperation = op.SetHash(op.GenerateHash())

	return op, nil
}
