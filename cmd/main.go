package main

import (
	"encoding/json"
	"os"
	"path"

	"ledger-sats-stack/pkg/config"
	"ledger-sats-stack/pkg/httpd"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

func main() {
	log.SetFormatter(&prefixed.TextFormatter{
		TimestampFormat:  "2006/01/02 - 15:04:05",
		FullTimestamp:    true,
		QuoteEmptyFields: true,
		SpacePadding:     45,
	})

	configuration := loadConfig()

	xrpc := httpd.GetXRPC(
		*configuration.RPCURL,
		*configuration.RPCUser,
		*configuration.RPCPassword,
		configuration.RPCTLS,
	)
	defer xrpc.Shutdown()

	httpd.WaitForNodeSync(xrpc)

	_ = xrpc.ImportAccounts(configuration.Accounts)

	engine := httpd.GetRouter(xrpc)
	engine.Run(":20000")
}

func loadConfig() config.Configuration {
	home, err := homedir.Dir()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Cannot obtain user home directory")
	}

	configPath := path.Join(home, ".sats.json")

	file, err := os.Open(configPath)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Cannot open config file")
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	configuration := config.Configuration{}

	err = decoder.Decode(&configuration)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Cannot decode accounts config JSON")
	}

	configuration.Validate()

	log.WithFields(log.Fields{
		"path": configPath,
	}).Info("Loaded config file")

	return configuration
}
