package agent

import (
	"errors"
	"io"

	"github.com/findy-network/findy-agent/agent/cloud"
	"github.com/findy-network/findy-agent/agent/ssi"
	"github.com/findy-network/findy-agent/cmds"
	"github.com/lainio/err2"
)

type ExportCmd struct {
	cmds.Cmd

	WalletKeyLegacy bool

	Filename  string
	ExportKey string
}

func (c ExportCmd) Validate() error {
	if !c.WalletKeyLegacy {
		if err := c.Cmd.Validate(); err != nil {
			return err
		}
		if err := c.Cmd.ValidateWalletExistence(true); err != nil {
			return err
		}
	} else {
		exists := ssi.NewWalletCfg(c.WalletName, c.WalletKey).Exists(false)
		if !exists {
			return errors.New("legacy wallet not exist")
		}
	}
	if c.Filename == "" {
		return errors.New("export path cannot be empty")
	}
	if err := cmds.ValidateKey(c.ExportKey, "export"); err != nil {
		return err
	}
	return nil
}

func (c ExportCmd) Exec(w io.Writer) (r Result, err error) {
	defer err2.Annotate("export wallet cmd", &err)

	agent := cloud.NewEA()
	wallet := *ssi.NewRawWalletCfg(c.WalletName, c.WalletKey)
	if c.WalletKeyLegacy {
		wallet = *ssi.NewWalletCfg(c.WalletName, c.WalletKey)
	}
	agent.OpenWallet(wallet)
	defer agent.CloseWallet()

	agent.ExportWallet(c.ExportKey, c.Filename)
	err2.Check(agent.Export.Result().Err())

	cmds.Fprintln(w, "wallet exported:", c.Filename)
	return Result{}, nil
}
