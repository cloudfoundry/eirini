package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/cloudfoundry-incubator/credhub-cli/commands"
	"github.com/cloudfoundry-incubator/credhub-cli/config"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/jessevdk/go-flags"
)

type NeedsClient interface {
	SetClient(*credhub.CredHub)
}
type NeedsConfig interface {
	SetConfig(config.Config)
}

func main() {
	debug.SetTraceback("all")
	parser := flags.NewParser(&commands.CredHub, flags.HelpFlag)
	parser.SubcommandsOptional = true
	parser.CommandHandler = func(command flags.Commander, args []string) error {
		if command == nil {
			parser.WriteHelp(os.Stderr)
			os.Exit(1)
		}

		if cmd, ok := command.(NeedsConfig); ok {
			cmd.SetConfig(config.ReadConfig())
		}

		if cmd, ok := command.(NeedsClient); ok {
			cfg := config.ReadConfig()
			if err := config.ValidateConfig(cfg); err != nil {
				return err
			}
			client, err := credhub.New(cfg.ApiURL,
				credhub.AuthURL(cfg.AuthURL),
				credhub.CaCerts(cfg.CaCerts...),
				credhub.SkipTLSValidation(cfg.InsecureSkipVerify),
				credhub.Auth(auth.Uaa(
					cfg.ClientID,
					cfg.ClientSecret,
					"",
					"",
					cfg.AccessToken,
					cfg.RefreshToken,
					true,
				)))
			if err != nil {
				return err
			}
			cmd.SetClient(client)
		}
		return command.Execute(args)
	}

	_, err := parser.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
