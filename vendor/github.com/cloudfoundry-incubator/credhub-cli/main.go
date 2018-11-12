package main // import "code.cloudfoundry.org/credhub-cli"

import (
	"fmt"
	"os"
	"runtime/debug"

	"code.cloudfoundry.org/credhub-cli/commands"
	"code.cloudfoundry.org/credhub-cli/config"
	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/auth"
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
			clientId := cfg.ClientID
			clientSecret := cfg.ClientSecret
			useClientCredentials := true
			if clientId == "" {
				clientId = config.AuthClient
				clientSecret = config.AuthPassword
				useClientCredentials = false
			}
			client, err := credhub.New(cfg.ApiURL,
				credhub.AuthURL(cfg.AuthURL),
				credhub.CaCerts(cfg.CaCerts...),
				credhub.SkipTLSValidation(cfg.InsecureSkipVerify),
				credhub.Auth(auth.Uaa(
					clientId,
					clientSecret,
					"",
					"",
					cfg.AccessToken,
					cfg.RefreshToken,
					useClientCredentials,
				)),
				credhub.ServerVersion(cfg.ServerVersion),
			)
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
