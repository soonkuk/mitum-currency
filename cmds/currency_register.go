package cmds

import (
	"github.com/spikeekips/mitum-currency/currency"
	"github.com/spikeekips/mitum/base"
	"github.com/spikeekips/mitum/base/operation"
	"github.com/spikeekips/mitum/util"
	"golang.org/x/xerrors"
)

type CurrencyFixedFeeerFlags struct {
	Receiver AddressFlag `name:"receiver" help:"fee receiver account address"`
	Amount   BigFlag     `name:"amount" help:"fee amount"`
	feeer    currency.Feeer
}

func (fl *CurrencyFixedFeeerFlags) IsValid([]byte) error {
	if len(fl.Receiver.String()) < 1 {
		return nil
	}

	var receiver base.Address
	if a, err := fl.Receiver.Encode(jenc); err != nil {
		return xerrors.Errorf("invalid receiver format, %q: %w", fl.Receiver.String(), err)
	} else if err := a.IsValid(nil); err != nil {
		return xerrors.Errorf("invalid receiver address, %q: %w", fl.Receiver.String(), err)
	} else {
		receiver = a
	}

	feeer := currency.NewFixedFeeer(receiver, fl.Amount.Big)
	if err := feeer.IsValid(nil); err != nil {
		return err
	} else {
		fl.feeer = feeer
	}

	return nil
}

type CurrencyRatioFeeerFlags struct {
	Receiver AddressFlag `name:"receiver" help:"fee receiver account address"`
	Ratio    float64     `name:"ratio" help:"fee ratio, multifly by operation amount"`
	Min      BigFlag     `name:"min" help:"minimum fee"`
	Max      BigFlag     `name:"max" help:"maximum fee"`
	feeer    currency.Feeer
}

func (fl *CurrencyRatioFeeerFlags) IsValid([]byte) error {
	if len(fl.Receiver.String()) < 1 {
		return nil
	}

	var receiver base.Address
	if a, err := fl.Receiver.Encode(jenc); err != nil {
		return xerrors.Errorf("invalid receiver format, %q: %w", fl.Receiver.String(), err)
	} else if err := a.IsValid(nil); err != nil {
		return xerrors.Errorf("invalid receiver address, %q: %w", fl.Receiver.String(), err)
	} else {
		receiver = a
	}

	feeer := currency.NewRatioFeeer(receiver, fl.Ratio, fl.Min.Big, fl.Max.Big)
	if err := feeer.IsValid(nil); err != nil {
		return err
	} else {
		fl.feeer = feeer
	}

	return nil
}

type CurrencyPolicyFlags struct {
	NewAccountMinBalance BigFlag `name:"new-account-min-balance" help:"minimum balance for new account"` // nolint lll
}

func (fl *CurrencyPolicyFlags) IsValid([]byte) error {
	return nil
}

type CurrencyDesignFlags struct {
	Currency                CurrencyIDFlag `arg:"" name:"currency-id" help:"currency id" required:""`
	GenesisAmount           BigFlag        `arg:"" name:"genesis-amount" help:"genesis amount" required:""`
	GenesisAccount          AddressFlag    `arg:"" name:"genesis-account" help:"genesis-account address for genesis balance" required:""` // nolint lll
	CurrencyPolicyFlags     `prefix:"policy-" help:"currency policy" required:""`
	FeeerString             string `name:"feeer" help:"feeer type, {nil, fixed, ratio}" required:""`
	CurrencyFixedFeeerFlags `prefix:"feeer-fixed-" help:"fixed feeer"`
	CurrencyRatioFeeerFlags `prefix:"feeer-ratio-" help:"ratio feeer"`
	currencyDesign          currency.CurrencyDesign
}

func (fl *CurrencyDesignFlags) IsValid([]byte) error {
	if err := fl.CurrencyPolicyFlags.IsValid(nil); err != nil {
		return err
	} else if err := fl.CurrencyFixedFeeerFlags.IsValid(nil); err != nil {
		return err
	} else if err := fl.CurrencyRatioFeeerFlags.IsValid(nil); err != nil {
		return err
	}

	var feeer currency.Feeer
	switch t := fl.FeeerString; t {
	case currency.FeeerNil, "":
		feeer = currency.NewNilFeeer()
	case currency.FeeerFixed:
		feeer = fl.CurrencyFixedFeeerFlags.feeer
	case currency.FeeerRatio:
		feeer = fl.CurrencyRatioFeeerFlags.feeer
	default:
		return xerrors.Errorf("unknown feeer type, %q", t)
	}

	if feeer == nil {
		return xerrors.Errorf("empty feeer flags")
	} else if err := feeer.IsValid(nil); err != nil {
		return err
	}

	po := currency.NewCurrencyPolicy(fl.CurrencyPolicyFlags.NewAccountMinBalance.Big, feeer)
	if err := po.IsValid(nil); err != nil {
		return err
	}

	var genesisAccount base.Address
	if a, err := fl.GenesisAccount.Encode(jenc); err != nil {
		return xerrors.Errorf("invalid genesis-account format, %q: %w", fl.GenesisAccount.String(), err)
	} else if err := a.IsValid(nil); err != nil {
		return xerrors.Errorf("invalid genesis-account address, %q: %w", fl.GenesisAccount.String(), err)
	} else {
		genesisAccount = a
	}

	am := currency.NewAmount(fl.GenesisAmount.Big, fl.Currency.CID)
	if err := am.IsValid(nil); err != nil {
		return err
	}

	de := currency.NewCurrencyDesign(am, genesisAccount, po)
	if err := de.IsValid(nil); err != nil {
		return err
	} else {
		fl.currencyDesign = de
	}

	return nil
}

type CurrencyRegisterCommand struct {
	*BaseCommand
	OperationFlags
	CurrencyDesignFlags
}

func NewCurrencyRegisterCommand() CurrencyRegisterCommand {
	return CurrencyRegisterCommand{
		BaseCommand: NewBaseCommand("currency-register-operation"),
	}
}

func (cmd *CurrencyRegisterCommand) Run(version util.Version) error { // nolint:dupl
	if err := cmd.Initialize(cmd, version); err != nil {
		return xerrors.Errorf("failed to initialize command: %w", err)
	}

	if err := cmd.parseFlags(); err != nil {
		return err
	}

	var op operation.Operation
	if i, err := cmd.createOperation(); err != nil {
		return xerrors.Errorf("failed to create currency-register operation: %w", err)
	} else if err := i.IsValid([]byte(cmd.OperationFlags.NetworkID)); err != nil {
		return xerrors.Errorf("invalid currency-register operation: %w", err)
	} else {
		cmd.Log().Debug().Interface("operation", i).Msg("operation loaded")

		op = i
	}

	if i, err := operation.NewBaseSeal(
		cmd.OperationFlags.Privatekey,
		[]operation.Operation{op},
		[]byte(cmd.OperationFlags.NetworkID),
	); err != nil {
		return xerrors.Errorf("failed to create operation.Seal: %w", err)
	} else {
		cmd.Log().Debug().Interface("seal", i).Msg("seal loaded")

		cmd.pretty(cmd.Pretty, i)
	}

	return nil
}

func (cmd *CurrencyRegisterCommand) parseFlags() error {
	if err := cmd.OperationFlags.IsValid(nil); err != nil {
		return err
	} else if err := cmd.CurrencyDesignFlags.IsValid(nil); err != nil {
		return err
	}

	cmd.Log().Debug().Interface("currency-design", cmd.CurrencyDesignFlags.currencyDesign).Msg("currency design loaded")

	return nil
}

func (cmd *CurrencyRegisterCommand) createOperation() (currency.CurrencyRegister, error) {
	fact := currency.NewCurrencyRegisterFact([]byte(cmd.Token), cmd.currencyDesign)

	var fs []operation.FactSign
	if sig, err := operation.NewFactSignature(
		cmd.OperationFlags.Privatekey,
		fact,
		[]byte(cmd.OperationFlags.NetworkID),
	); err != nil {
		return currency.CurrencyRegister{}, err
	} else {
		fs = append(fs, operation.NewBaseFactSign(cmd.OperationFlags.Privatekey.Publickey(), sig))
	}

	return currency.NewCurrencyRegister(fact, fs, cmd.OperationFlags.Memo)
}
