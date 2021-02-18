/*
Copyright © 2021 alexander comerford alex@taoa.io

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"fmt"
	"strings"
	"net/url"
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/algorand/go-algorand-sdk/crypto"
	"github.com/algorand/go-algorand-sdk/future"
	"github.com/algorand/go-algorand-sdk/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/client/kmd"
	"github.com/algorand/go-algorand-sdk/mnemonic"
	"github.com/algorand/go-algorand-sdk/types"
)

const envPrefix = "ALGORAND_CLAIMER"

var AlgorandClaimerConfig struct {
	Host              string
	AlgodPort         string
	AlgodToken        string
	KmdPort           string
	KmdToken          string
	WalletName        string
	Mnemonic         string
	Passphrase        string
	DeleteKeyOnExit   bool
	MinClaimAmount    int // In microalgos
}

func init() {

	rootCmd.Flags().StringVarP(&AlgorandClaimerConfig.Host, "host", "H", "localhost", "Host of Kmd and Algod")
	rootCmd.Flags().StringVarP(&AlgorandClaimerConfig.AlgodPort, "algod-port", "", "8080", "Port of running algod instance")
	rootCmd.Flags().StringVarP(&AlgorandClaimerConfig.AlgodToken, "algod-token", "", "", "Access token for algod instance")
	rootCmd.Flags().StringVarP(&AlgorandClaimerConfig.KmdPort, "kmd-port", "", "7833", "Port of running kmd instance")
	rootCmd.Flags().StringVarP(&AlgorandClaimerConfig.KmdToken, "kmd-token", "", "", "Access token for kmd instance")
	rootCmd.Flags().StringVarP(&AlgorandClaimerConfig.WalletName, "wallet-name", "w", "claim", "Name of the wallet")
	rootCmd.Flags().StringVarP(&AlgorandClaimerConfig.Mnemonic, "mnemonic", "m", "", "Mnemonic to recover wallet")
	rootCmd.Flags().StringVarP(&AlgorandClaimerConfig.Passphrase, "passphrase", "p", "", "Passphrase for wallet")
	rootCmd.Flags().BoolVarP(&AlgorandClaimerConfig.DeleteKeyOnExit, "delete-key-on-exit", "d", true, "Delete local wallet storage on exit")
	rootCmd.Flags().IntVarP(&AlgorandClaimerConfig.MinClaimAmount, "min-claim-amount", "c", 0, "Minimum amount of microalgos to trigger the transaction")

	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	bindFlags(rootCmd, v)
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			v.BindEnv(f.Name, fmt.Sprintf("%s_%s", envPrefix, envVarSuffix))
		}
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func main() {
	Execute()
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "algorand-claimer",
	Short: "Claim staked ALGO by sending a zero transaction to yourself",
	Long: `In Algorand, a holder of ALGO automatically accumulates rewards since the
 most recent recorded balance on the blockchain. Unfortunately these rewards do not
 automatically compound and will only compound if transactions occur frequently to/from
 an address. This script is meant to automatically send 0 transactions to an address
 effectively claiming accrued rewards.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	RunE: func(cmd *cobra.Command, args []string) error {
		return Claim();
	},
}

type AlgorandClaimer struct {
	kmdClient          kmd.Client
	algodClient        algod.Client
	wallet             kmd.APIV1Wallet
	address            string
	walletHandleToken  string
}

func NewAlgorandClaimer() (*AlgorandClaimer, error) {
	return &AlgorandClaimer{}, nil
}

func (ac *AlgorandClaimer) InitializeKmdClient(kmdToken string) error {
	// Construct the kmd url to communicate with
	kmdAddress := (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%s", AlgorandClaimerConfig.Host, AlgorandClaimerConfig.KmdPort),
	}).String()
	
	// Create kmd client
	kmdClient, err := kmd.MakeClient(kmdAddress, kmdToken)
	if err != nil {		
		log.Error(fmt.Sprintf("failed to make kmd client: %s\n", err))
		return err
	}

	ac.kmdClient = kmdClient

	return nil
}

func (ac *AlgorandClaimer) InitializeAlgodClient(algodToken string) error {
	// Construct the algod url to communicate with
	algodAddress := (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%s", AlgorandClaimerConfig.Host, AlgorandClaimerConfig.AlgodPort),
	}).String()

	// Create algod client
	algodClient, err := algod.MakeClient(algodAddress, algodToken)
	if err != nil {
		log.Error(fmt.Sprintf("failed to make algod client: %s\n", err))
		return err
	}

	ac.algodClient = *algodClient

	return nil
}

func (ac *AlgorandClaimer) RecoverWallet(walletName string, Mnemonic string, passphrase string) error {

	// Get the list of wallets
	listResponse, err := ac.kmdClient.ListWallets()
	if err != nil {
		log.Error(fmt.Sprintf("error listing wallets: %s\n", err))
		return err
	}

	// Find wallet in list of wallets
	log.WithFields(log.Fields{
    "num_wallets": len(listResponse.Wallets),
  }).Info("Found some wallets")
	for _, wallet := range listResponse.Wallets {
		if wallet.Name == walletName {
			log.WithFields(log.Fields{
				"Name": wallet.Name,
				"ID": wallet.ID,
			}).Info("Found pre-existing desired wallet")

			ac.wallet = wallet
			break
		}
	}
	
	// Create wallet if it hasn't been found
	if ac.wallet.ID == "" {
		log.WithFields(log.Fields{
			"Name": walletName,
		}).Warning("No pre-existing wallet found, recovering ...")

		// Create wallet key from backup mnemonic
		keyBytes, err := mnemonic.ToKey(Mnemonic)
		if err != nil {
			log.Error(fmt.Sprintf("failed to get key: %s\n", err))
			return err
		}
		
		var mdk types.MasterDerivationKey
		copy(mdk[:], keyBytes)
		log.WithFields(log.Fields{
			"Name": walletName,
		}).Info("Creating wallet ...")
		cwResponse, err := ac.kmdClient.CreateWallet(walletName, passphrase, kmd.DefaultWalletDriver, mdk)
		if err != nil {
			log.Error(fmt.Sprintf("error creating wallet: %s\n", err))
			return err
		}
		log.WithFields(log.Fields{
			"Name": cwResponse.Wallet.Name,
			"ID": cwResponse.Wallet.ID,
		}).Info("Created wallet")

		ac.wallet = cwResponse.Wallet
	}

	return nil
}

func (ac *AlgorandClaimer) InitializeWalletHandleToken(passphrase string) error {
	// Get a wallet handle
	walletHandle, err := ac.kmdClient.InitWalletHandle(ac.wallet.ID, passphrase)
	if err != nil {
		log.Error(fmt.Sprintf("Error initializing wallet handle: %s\n", err))
		return err
	}

	// Extract the wallet handle
	ac.walletHandleToken = walletHandle.WalletHandleToken
	
	log.WithFields(log.Fields{
		"Token": ac.walletHandleToken,
	}).Info("Obtained wallet token")

	return nil
}

func (ac *AlgorandClaimer) RecoverKey(Mnemonic string, passphrase string) error {

	// Initialize the wallet token to be used further
	err := ac.InitializeWalletHandleToken(passphrase)
	if err != nil {
		return err
	}

	// Generate the private key from the mnemonic
	pk, err := mnemonic.ToPrivateKey(Mnemonic)
	if err != nil {
		log.Error(fmt.Sprintf("Error generating private key: %s\n", err))
		return err
	}

	// Create an address from the private key
	pkaddress, err := crypto.GenerateAddressFromSK(pk)
	if err != nil {
		log.Error(fmt.Sprintf("Error generating address: %s\n", err))
		return err				
	}

	// List all keys provided from the kmd daemon
	keys, err := ac.kmdClient.ListKeys(ac.walletHandleToken)
	if err != nil {
		log.Error(fmt.Sprintf("Error getting keys: %s\n", err))
		return err
	}
	log.WithFields(log.Fields{
		"num_addresses": len(keys.Addresses),
	}).Info("Found some addresses")

	// Check if the address already exists
	for _, kaddress := range keys.Addresses {
		if kaddress == pkaddress.String() {
			log.WithFields(log.Fields{
				"Address": kaddress,
			}).Info("Found pre-existing desired address")

			ac.address = kaddress
			break
		}
	}

	// Import the key if it doesn't exist
	if ac.address == "" {
		log.Info("No pre-existing key found, importing ...")
		ikr, err := ac.kmdClient.ImportKey(ac.walletHandleToken, pk)
		if err != nil {
			log.Error(fmt.Sprintf("Error importing key: %s\n", err))
			return err
		}
		ac.address = pkaddress.String()
		log.WithFields(log.Fields{
			"Address": ikr.Address,
		}).Info("Imported address")
	}
	
	return nil
}

func (ac *AlgorandClaimer) DeleteKey(passphrase string) error {
	// Initialize the wallet token to be used further
	err := ac.InitializeWalletHandleToken(passphrase)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Address": ac.address,
	}).Info("Deleting address")
	_, err = ac.kmdClient.DeleteKey(ac.walletHandleToken, passphrase, ac.address)
	if err != nil {
		return err
	}
	log.Info("Deleted address")
	
	return nil
}

func (ac *AlgorandClaimer) ClaimRewards(minClaimAmount int, passphrase string) error {

	// Get the account information, including balance and rewards
	accountInfo, err := ac.algodClient.AccountInformation(ac.address).Do(context.Background())
	if err != nil {
		log.Error(fmt.Sprintf("Error getting account information: %s\n", err))
		return err
	}

	// Check if the pending rewards is worth claiming
	if accountInfo.PendingRewards > uint64(minClaimAmount) && uint64(minClaimAmount) > 0 {		
		log.WithFields(log.Fields{
			"PendingRewards": accountInfo.PendingRewards,
		}).Info("Claiming rewards")

		// Get the defaults for a transaction
		txParams, err := ac.algodClient.SuggestedParams().Do(context.Background())
		if err != nil {
			return err
		}

		log.Info("Building transaction")
		tx, err := future.MakePaymentTxn(ac.address, ac.address, 0, nil, "", txParams)
		if err != nil {
			fmt.Printf("Error creating transaction: %s\n", err)
			return err
		}
		
		// Sign the transaction
		log.Info("Signing transaction")
		signResponse, err := ac.kmdClient.SignTransaction(ac.walletHandleToken, passphrase, tx)
		if err != nil {
			fmt.Printf("Failed to sign transaction with kmd: %s\n", err)
			return err
		}

		log.Info("Sending transaction")
		txid, err := ac.algodClient.SendRawTransaction(signResponse.SignedTransaction).Do(context.Background())
		if err != nil {
			fmt.Printf("failed to send transaction: %s\n", err)
			return err
		}

		log.WithFields(log.Fields{
			"TxnID": txid,
		}).Info("Transaction Sent")
	} else {
		log.Info("Pending rewards not worth claiming, exiting ...")
	}

	return nil
}

func Claim() error {
	claimer, _ := NewAlgorandClaimer()

	// Initialize clients
	err := claimer.InitializeKmdClient(AlgorandClaimerConfig.KmdToken)
	if err != nil {
		return err
	}
	err = claimer.InitializeAlgodClient(AlgorandClaimerConfig.AlgodToken)
	if err != nil {
		return err
	}

	// Recover the wallet from the Mnemonic and add a passphrase
	err = claimer.RecoverWallet(
		AlgorandClaimerConfig.WalletName,
		AlgorandClaimerConfig.Mnemonic,
		AlgorandClaimerConfig.Passphrase,
	)
	if err != nil {
		return err
	}

	// Recover the Key and associated address
	err = claimer.RecoverKey(
		AlgorandClaimerConfig.Mnemonic,
		AlgorandClaimerConfig.Passphrase,
	)
	if err != nil {
		return err
	}

	// If the delete-on-exit command is flipped, delete on exit
	if AlgorandClaimerConfig.DeleteKeyOnExit {
		defer claimer.DeleteKey(AlgorandClaimerConfig.Passphrase)
	}

	// Recover the Key and associated address
	err = claimer.ClaimRewards(
		AlgorandClaimerConfig.MinClaimAmount,
		AlgorandClaimerConfig.Passphrase,
	)
	if err != nil {
		return err
	}
	return nil
}
