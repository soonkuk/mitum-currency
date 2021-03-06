package digest

import (
	"bytes"
	"time"

	"github.com/spikeekips/mitum-currency/currency"
	"github.com/spikeekips/mitum/base"
	"github.com/spikeekips/mitum/base/key"
	"github.com/spikeekips/mitum/base/operation"
	"github.com/spikeekips/mitum/util/encoder"
	"github.com/spikeekips/mitum/util/hint"
	"github.com/spikeekips/mitum/util/localtime"
	"golang.org/x/xerrors"
)

var (
	templatePrivateKeyString = "Kzxb3TcaxHCp9iq6ekyaNjaeRSdqzvv9JrazTV8cVsZq9U2FQSSG"
	templatePublickey        key.Publickey
	templateCurrencyID       = currency.CurrencyID("xXx")
	templateSender           = currency.Address("mother")
	templateReceiver         = currency.Address("father")
	templateToken            = []byte("raised by")
	templateSignature        = key.Signature([]byte("wolves"))
	templateBig              = currency.NewBig(-333)
	templateSignedAtString   = "2020-10-08T07:53:26Z"
	templateSignedAt         time.Time
)

func init() {
	if priv, err := key.NewBTCPrivatekeyFromString(templatePrivateKeyString); err != nil {
		panic(err)
	} else {
		templatePublickey = priv.Publickey()
	}

	templateSignedAt, _ = time.Parse(time.RFC3339, templateSignedAtString)
}

type Builder struct {
	enc       encoder.Encoder
	networkID base.NetworkID
}

func NewBuilder(enc encoder.Encoder, networkID base.NetworkID) Builder {
	return Builder{enc: enc, networkID: networkID}
}

func (bl Builder) FactTemplate(ht hint.Hint) (Hal, error) {
	switch ht.Type() {
	case currency.CreateAccountsType:
		return bl.templateCreateAccountsFact(), nil
	case currency.KeyUpdaterType:
		return bl.templateKeyUpdaterFact(), nil
	case currency.TransfersType:
		return bl.templateTransfersFact(), nil
	case currency.CurrencyRegisterType:
		return bl.templateCurrencyRegisterFact(), nil
	case currency.CurrencyPolicyUpdaterType:
		return bl.templateCurrencyPolicyUpdaterFact(), nil
	default:
		return nil, xerrors.Errorf("unknown operation, %v", ht.Verbose())
	}
}

func (bl Builder) templateCreateAccountsFact() Hal {
	nkey, _ := currency.NewKey(templatePublickey, 100)
	nkeys, _ := currency.NewKeys([]currency.Key{nkey}, 100)

	fact := currency.NewCreateAccountsFact(
		templateToken,
		templateSender,
		[]currency.CreateAccountsItem{currency.NewCreateAccountsItemSingleAmount(
			nkeys,
			currency.NewAmount(templateBig, templateCurrencyID),
		)},
	)

	hal := NewBaseHal(fact, HalLink{})
	return hal.AddExtras("default", map[string]interface{}{
		"token":               templateToken,
		"sender":              templateSender,
		"items.keys.keys.key": templatePublickey,
		"items.big":           templateBig,
		"currency":            templateCurrencyID,
	})
}

func (bl Builder) templateKeyUpdaterFact() Hal {
	nkey, _ := currency.NewKey(templatePublickey, 100)
	nkeys, _ := currency.NewKeys([]currency.Key{nkey}, 100)

	fact := currency.NewKeyUpdaterFact(
		templateToken,
		templateSender,
		nkeys,
		templateCurrencyID,
	)

	hal := NewBaseHal(fact, HalLink{})
	return hal.AddExtras("default", map[string]interface{}{
		"token":         templateToken,
		"target":        templateSender,
		"keys.keys.key": templatePublickey,
		"currency":      templateCurrencyID,
	})
}

func (bl Builder) templateTransfersFact() Hal {
	fact := currency.NewTransfersFact(
		templateToken,
		templateSender,
		[]currency.TransfersItem{currency.NewTransfersItemSingleAmount(
			templateReceiver,
			currency.NewAmount(templateBig, templateCurrencyID),
		)},
	)

	hal := NewBaseHal(fact, HalLink{})

	return hal.AddExtras("default", map[string]interface{}{
		"token":          templateToken,
		"sender":         templateSender,
		"items.receiver": templateReceiver,
		"items.big":      templateBig,
		"items.currency": templateCurrencyID,
	})
}

func (bl Builder) templateCurrencyRegisterFact() Hal {
	po := currency.NewCurrencyPolicy(templateBig, currency.NewNilFeeer())
	de := currency.NewCurrencyDesign(
		currency.NewAmount(templateBig, templateCurrencyID),
		templateReceiver,
		po,
	)
	fact := currency.NewCurrencyRegisterFact(templateToken, de)

	hal := NewBaseHal(fact, HalLink{})

	return hal.AddExtras("default", map[string]interface{}{
		"token":                    templateToken,
		"amount.amount":            templateBig,
		"amount.currency":          templateCurrencyID,
		"currency.genesis_account": templateReceiver,
		"currency.policy.new_account_min_balance": templateBig,
	})
}

func (bl Builder) templateCurrencyPolicyUpdaterFact() Hal {
	po := currency.NewCurrencyPolicy(templateBig, currency.NewNilFeeer())
	fact := currency.NewCurrencyPolicyUpdaterFact(templateToken, templateCurrencyID, po)

	hal := NewBaseHal(fact, HalLink{})

	return hal.AddExtras("default", map[string]interface{}{
		"token":                          templateToken,
		"currency":                       templateCurrencyID,
		"policy.new_account_min_balance": templateBig,
	})
}

func (bl Builder) BuildFact(b []byte) (Hal, error) {
	var fact base.Fact
	if hinter, err := bl.enc.DecodeByHint(b); err != nil {
		return nil, err
	} else if f, ok := hinter.(base.Fact); !ok {
		return nil, xerrors.Errorf("not base.Fact, %T", hinter)
	} else {
		fact = f
	}

	switch t := fact.(type) {
	case currency.CreateAccountsFact:
		return bl.buildFactCreateAccounts(t)
	case currency.KeyUpdaterFact:
		return bl.buildFactKeyUpdater(t)
	case currency.TransfersFact:
		return bl.buildFactTransfers(t)
	case currency.CurrencyRegisterFact:
		return bl.buildFactCurrencyRegister(t)
	case currency.CurrencyPolicyUpdaterFact:
		return bl.buildFactCurrencyPolicyUpdater(t)
	default:
		return nil, xerrors.Errorf("unknown fact, %T", fact)
	}
}

func (bl Builder) buildFactCreateAccounts(fact currency.CreateAccountsFact) (Hal, error) {
	var token []byte
	if t, err := bl.checkToken(fact.Token()); err != nil {
		return nil, err
	} else {
		token = t
	}

	items := make([]currency.CreateAccountsItem, len(fact.Items()))
	for i := range fact.Items() {
		item := fact.Items()[i]
		if len(item.Amounts()) < 1 {
			return nil, xerrors.Errorf("empty Amounts")
		}

		if ks, err := currency.NewKeys(item.Keys().Keys(), item.Keys().Threshold()); err != nil {
			return nil, err
		} else {
			items[i] = currency.NewCreateAccountsItemSingleAmount(ks, item.Amounts()[0])
		}
	}

	nfact := currency.NewCreateAccountsFact(token, fact.Sender(), items)
	nfact = nfact.Rebulild()
	if err := bl.isValidFactCreateAccounts(nfact); err != nil {
		return nil, err
	}

	var hal Hal
	hal = NewBaseHal(nil, HalLink{})
	if op, err := currency.NewCreateAccounts(
		nfact,
		[]operation.FactSign{
			operation.RawBaseFactSign(templatePublickey, templateSignature, templateSignedAt),
		},
		"",
	); err != nil {
		return nil, err
	} else {
		hal = hal.SetInterface(op)
	}

	return hal.
		AddExtras("default", map[string]interface{}{
			"fact_signs.signer":    templatePublickey,
			"fact_signs.signature": templateSignature,
		}).
		AddExtras("signature_base", operation.NewBytesForFactSignature(nfact, bl.networkID)), nil
}

func (bl Builder) buildFactKeyUpdater(fact currency.KeyUpdaterFact) (Hal, error) {
	var token []byte
	if t, err := bl.checkToken(fact.Token()); err != nil {
		return nil, err
	} else {
		token = t
	}

	var ks currency.Keys
	if k, err := currency.NewKeys(fact.Keys().Keys(), fact.Keys().Threshold()); err != nil {
		return nil, err
	} else {
		ks = k
	}

	nfact := currency.NewKeyUpdaterFact(token, fact.Target(), ks, fact.Currency())
	if err := bl.isValidFactKeyUpdater(nfact); err != nil {
		return nil, err
	}

	var hal Hal
	hal = NewBaseHal(nil, HalLink{})
	if op, err := currency.NewKeyUpdater(
		nfact,
		[]operation.FactSign{
			operation.RawBaseFactSign(templatePublickey, templateSignature, templateSignedAt),
		},
		"",
	); err != nil {
		return nil, err
	} else {
		hal = hal.SetInterface(op)
	}

	return hal.
		AddExtras("default", map[string]interface{}{
			"fact_signs.signer":    templatePublickey,
			"fact_signs.signature": templateSignature,
		}).
		AddExtras("signature_base", operation.NewBytesForFactSignature(nfact, bl.networkID)), nil
}

func (bl Builder) buildFactTransfers(fact currency.TransfersFact) (Hal, error) {
	var token []byte
	if t, err := bl.checkToken(fact.Token()); err != nil {
		return nil, err
	} else {
		token = t
	}

	nfact := currency.NewTransfersFact(token, fact.Sender(), fact.Items())
	nfact = nfact.Rebulild()
	if err := bl.isValidFactTransfers(nfact); err != nil {
		return nil, err
	}

	var hal Hal
	hal = NewBaseHal(nil, HalLink{})
	if op, err := currency.NewTransfers(
		nfact,
		[]operation.FactSign{
			operation.RawBaseFactSign(templatePublickey, templateSignature, templateSignedAt),
		},
		"",
	); err != nil {
		return nil, err
	} else {
		hal = hal.SetInterface(op)
	}

	return hal.
		AddExtras("default", map[string]interface{}{
			"fact_signs.signer":    templatePublickey,
			"fact_signs.signature": templateSignature,
		}).
		AddExtras("signature_base", operation.NewBytesForFactSignature(nfact, bl.networkID)), nil
}

func (bl Builder) buildFactCurrencyRegister(fact currency.CurrencyRegisterFact) (Hal, error) {
	var token []byte
	if t, err := bl.checkToken(fact.Token()); err != nil {
		return nil, err
	} else {
		token = t
	}

	nfact := currency.NewCurrencyRegisterFact(token, fact.Currency())
	if err := bl.isValidFactCurrencyRegister(nfact); err != nil {
		return nil, err
	}

	var hal Hal
	hal = NewBaseHal(nil, HalLink{})
	if op, err := currency.NewCurrencyRegister(
		nfact,
		[]operation.FactSign{
			operation.RawBaseFactSign(templatePublickey, templateSignature, templateSignedAt),
		},
		"",
	); err != nil {
		return nil, err
	} else {
		hal = hal.SetInterface(op)
	}

	return hal.
		AddExtras("default", map[string]interface{}{
			"fact_signs.signer":    templatePublickey,
			"fact_signs.signature": templateSignature,
		}).
		AddExtras("signature_base", operation.NewBytesForFactSignature(nfact, bl.networkID)), nil
}

func (bl Builder) buildFactCurrencyPolicyUpdater(fact currency.CurrencyPolicyUpdaterFact) (Hal, error) {
	var token []byte
	if t, err := bl.checkToken(fact.Token()); err != nil {
		return nil, err
	} else {
		token = t
	}

	nfact := currency.NewCurrencyPolicyUpdaterFact(token, fact.Currency(), fact.Policy())
	if err := bl.isValidFactCurrencyPolicyUpdater(nfact); err != nil {
		return nil, err
	}

	var hal Hal
	hal = NewBaseHal(nil, HalLink{})
	if op, err := currency.NewCurrencyPolicyUpdater(
		nfact,
		[]operation.FactSign{
			operation.RawBaseFactSign(templatePublickey, templateSignature, templateSignedAt),
		},
		"",
	); err != nil {
		return nil, err
	} else {
		hal = hal.SetInterface(op)
	}

	return hal.
		AddExtras("default", map[string]interface{}{
			"fact_signs.signer":    templatePublickey,
			"fact_signs.signature": templateSignature,
		}).
		AddExtras("signature_base", operation.NewBytesForFactSignature(nfact, bl.networkID)), nil
}

func (bl Builder) isValidFactCreateAccounts(fact currency.CreateAccountsFact) error {
	if err := fact.IsValid(nil); err != nil {
		return err
	}

	if bytes.Equal(fact.Token(), templateToken) {
		return xerrors.Errorf("Please set token; token same with template default")
	}

	if fact.Sender().Equal(templateSender) {
		return xerrors.Errorf("Please set sender; sender is same with template default")
	}

	for i := range fact.Items() {
		if _, same := fact.Items()[i].Keys().Key(templatePublickey); same {
			return xerrors.Errorf("Please set key; key is same with template default")
		}
	}

	return nil
}

func (bl Builder) isValidFactKeyUpdater(fact currency.KeyUpdaterFact) error {
	if err := fact.IsValid(nil); err != nil {
		return err
	}

	if bytes.Equal(fact.Token(), templateToken) {
		return xerrors.Errorf("Please set token; token same with template default")
	}

	if fact.Target().Equal(templateSender) {
		return xerrors.Errorf("Please set target; target is same with template default")
	}

	if _, same := fact.Keys().Key(templatePublickey); same {
		return xerrors.Errorf("Please set key; key is same with template default")
	}

	return nil
}

func (bl Builder) isValidFactTransfers(fact currency.TransfersFact) error {
	if err := fact.IsValid(nil); err != nil {
		return err
	}

	if bytes.Equal(fact.Token(), templateToken) {
		return xerrors.Errorf("Please set token; token same with template default")
	}

	if fact.Sender().Equal(templateSender) {
		return xerrors.Errorf("Please set sender; sender is same with template default")
	}

	for i := range fact.Items() {
		if fact.Items()[i].Receiver().Equal(templateReceiver) {
			return xerrors.Errorf("Please set receiver; receiver is same with template default")
		}
	}

	return nil
}

func (bl Builder) isValidFactCurrencyRegister(fact currency.CurrencyRegisterFact) error {
	if err := fact.IsValid(nil); err != nil {
		return err
	}

	if bytes.Equal(fact.Token(), templateToken) {
		return xerrors.Errorf("Please set token; token same with template default")
	}

	if fact.Currency().GenesisAccount().Equal(templateReceiver) {
		return xerrors.Errorf("Please set genesis_account; genesis_account is same with template default")
	}

	if fact.Currency().Policy().NewAccountMinBalance().Equal(templateBig) {
		return xerrors.Errorf("Please set new_account_min_balance; new_account_min_balance is same with template default")
	}

	return nil
}

func (bl Builder) isValidFactCurrencyPolicyUpdater(fact currency.CurrencyPolicyUpdaterFact) error {
	if err := fact.IsValid(nil); err != nil {
		return err
	}

	if bytes.Equal(fact.Token(), templateToken) {
		return xerrors.Errorf("Please set token; token same with template default")
	}

	return nil
}

func (bl Builder) BuildOperation(b []byte) (Hal, error) {
	var op operation.Operation
	if hinter, err := bl.enc.DecodeByHint(b); err != nil {
		return nil, err
	} else if f, ok := hinter.(operation.Operation); !ok {
		return nil, xerrors.Errorf("not operation.Operation, %T", hinter)
	} else {
		op = f
	}

	var hal Hal
	if err := func() error {
		var err error
		switch t := op.(type) {
		case currency.CreateAccounts:
			hal, err = bl.buildCreateAccounts(t)
		case currency.KeyUpdater:
			hal, err = bl.buildKeyUpdater(t)
		case currency.Transfers:
			hal, err = bl.buildTransfers(t)
		case currency.CurrencyRegister:
			hal, err = bl.buildCurrencyRegister(t)
		case currency.CurrencyPolicyUpdater:
			hal, err = bl.buildCurrencyPolicyUpdater(t)
		default:
			return xerrors.Errorf("unknown operation.Operation, %T", t)
		}

		return err
	}(); err != nil {
		return nil, err
	}

	nop := hal.Interface().(operation.Operation)
	for i := range nop.Signs() {
		fs := nop.Signs()[i]
		if fs.Signer().Equal(templatePublickey) {
			return nil, xerrors.Errorf("Please set publickey; signer is same with template default")
		}

		if fs.Signature().Equal(templateSignature) {
			return nil, xerrors.Errorf("Please set signature; signature same with template default")
		}
	}

	return hal, nil
}

func (bl Builder) buildCreateAccounts(op currency.CreateAccounts) (Hal, error) {
	fs := bl.updateFactSigns(op.Signs())

	if nop, err := currency.NewCreateAccounts(op.Fact().(currency.CreateAccountsFact), fs, op.Memo); err != nil {
		return nil, err
	} else if err := nop.IsValid(bl.networkID); err != nil {
		return nil, err
	} else if err := bl.isValidFactCreateAccounts(nop.Fact().(currency.CreateAccountsFact)); err != nil {
		return nil, err
	} else {
		return NewBaseHal(nop, HalLink{}), nil
	}
}

func (bl Builder) buildKeyUpdater(op currency.KeyUpdater) (Hal, error) {
	fs := bl.updateFactSigns(op.Signs())

	if nop, err := currency.NewKeyUpdater(op.Fact().(currency.KeyUpdaterFact), fs, op.Memo); err != nil {
		return nil, err
	} else if err := nop.IsValid(bl.networkID); err != nil {
		return nil, err
	} else if err := bl.isValidFactKeyUpdater(nop.Fact().(currency.KeyUpdaterFact)); err != nil {
		return nil, err
	} else {
		return NewBaseHal(nop, HalLink{}), nil
	}
}

func (bl Builder) buildTransfers(op currency.Transfers) (Hal, error) {
	fs := bl.updateFactSigns(op.Signs())

	if nop, err := currency.NewTransfers(op.Fact().(currency.TransfersFact), fs, op.Memo); err != nil {
		return nil, err
	} else if err := nop.IsValid(bl.networkID); err != nil {
		return nil, err
	} else if err := bl.isValidFactTransfers(nop.Fact().(currency.TransfersFact)); err != nil {
		return nil, err
	} else {
		return NewBaseHal(nop, HalLink{}), nil
	}
}

func (bl Builder) buildCurrencyRegister(op currency.CurrencyRegister) (Hal, error) {
	fs := bl.updateFactSigns(op.Signs())

	if nop, err := currency.NewCurrencyRegister(op.Fact().(currency.CurrencyRegisterFact), fs, op.Memo); err != nil {
		return nil, err
	} else if err := nop.IsValid(bl.networkID); err != nil {
		return nil, err
	} else if err := bl.isValidFactCurrencyRegister(nop.Fact().(currency.CurrencyRegisterFact)); err != nil {
		return nil, err
	} else {
		return NewBaseHal(nop, HalLink{}), nil
	}
}

func (bl Builder) buildCurrencyPolicyUpdater(op currency.CurrencyPolicyUpdater) (Hal, error) {
	fs := bl.updateFactSigns(op.Signs())

	if nop, err := currency.NewCurrencyPolicyUpdater(
		op.Fact().(currency.CurrencyPolicyUpdaterFact),
		fs,
		op.Memo,
	); err != nil {
		return nil, err
	} else if err := nop.IsValid(bl.networkID); err != nil {
		return nil, err
	} else if err := bl.isValidFactCurrencyPolicyUpdater(nop.Fact().(currency.CurrencyPolicyUpdaterFact)); err != nil {
		return nil, err
	} else {
		return NewBaseHal(nop, HalLink{}), nil
	}
}

// checkToken checks token is valid; empty token will be updated with current
// time.
func (bl Builder) checkToken(token []byte) ([]byte, error) {
	if len(token) < 1 {
		return nil, xerrors.Errorf("empty token")
	}

	if bytes.Equal(token, templateToken) {
		return localtime.NewTime(localtime.UTCNow()).Bytes(), nil
	}

	return token, nil
}

// updateFactSigns regenerate the newly added factsign.
func (bl Builder) updateFactSigns(fss []operation.FactSign) []operation.FactSign {
	ufss := make([]operation.FactSign, len(fss))
	for i := range fss {
		fs := fss[i]

		if localtime.RFC3339(fs.SignedAt()) == localtime.RFC3339(templateSignedAt) {
			fs = operation.NewBaseFactSign(fs.Signer(), fs.Signature())
		}

		ufss[i] = fs
	}

	return ufss
}
