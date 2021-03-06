package currency

import (
	"golang.org/x/xerrors"

	"github.com/spikeekips/mitum/base"
	"github.com/spikeekips/mitum/base/key"
	"github.com/spikeekips/mitum/base/state"
	"github.com/spikeekips/mitum/util/valuehash"
)

func (op CurrencyPolicyUpdater) Process(
	func(key string) (state.State, bool, error),
	func(valuehash.Hash, ...state.State) error,
) error {
	// NOTE Process is nil func
	return nil
}

type CurrencyPolicyUpdaterProcessor struct {
	CurrencyPolicyUpdater
	cp        *CurrencyPool
	pubs      []key.Publickey
	threshold base.Threshold
	st        state.State
	de        CurrencyDesign
}

func NewCurrencyPolicyUpdaterProcessor(
	cp *CurrencyPool,
	pubs []key.Publickey,
	threshold base.Threshold,
) GetNewProcessor {
	return func(op state.Processor) (state.Processor, error) {
		if i, ok := op.(CurrencyPolicyUpdater); !ok {
			return nil, xerrors.Errorf("not CurrencyPolicyUpdater, %T", op)
		} else {
			return &CurrencyPolicyUpdaterProcessor{
				CurrencyPolicyUpdater: i,
				cp:                    cp,
				pubs:                  pubs,
				threshold:             threshold,
			}, nil
		}
	}
}

func (opp *CurrencyPolicyUpdaterProcessor) PreProcess(
	getState func(key string) (state.State, bool, error),
	_ func(valuehash.Hash, ...state.State) error,
) (state.Processor, error) {
	if len(opp.pubs) < 1 {
		return nil, xerrors.Errorf("empty publickeys for operation signs")
	} else if err := checkFactSignsByPubs(opp.pubs, opp.threshold, opp.Signs()); err != nil {
		return nil, err
	}

	fact := opp.Fact().(CurrencyPolicyUpdaterFact)

	if opp.cp != nil {
		if !opp.cp.Exists(fact.Currency()) {
			return nil, xerrors.Errorf("unknown currency, %q found", fact.Currency())
		}
	}

	if receiver := fact.Policy().Feeer().Receiver(); receiver != nil {
		if err := checkExistsState(StateKeyAccount(receiver), getState); err != nil {
			return nil, xerrors.Errorf("feeer receiver account not found: %w", err)
		}
	}

	switch st, found, err := getState(StateKeyCurrencyDesign(fact.Currency())); {
	case err != nil:
		return nil, err
	case !found:
		return nil, xerrors.Errorf("unknown currency, %q found", fact.Currency())
	default:
		opp.st = st

		if de, err := StateCurrencyDesignValue(st); err != nil {
			return nil, err
		} else {
			opp.de = de
		}
	}

	return opp, nil
}

func (opp *CurrencyPolicyUpdaterProcessor) Process(
	_ func(key string) (state.State, bool, error),
	setState func(valuehash.Hash, ...state.State) error,
) error {
	fact := opp.Fact().(CurrencyPolicyUpdaterFact)

	if i, err := SetStateCurrencyDesignValue(opp.st, opp.de.SetPolicy(fact.Policy())); err != nil {
		return err
	} else {
		return setState(fact.Hash(), i)
	}
}
