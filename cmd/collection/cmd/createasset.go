package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/algorand/go-algorand-sdk/crypto"
	"github.com/algorand/go-algorand-sdk/future"
	"github.com/algorand/go-algorand-sdk/types"

	"github.com/vecno-io/algo-collection/shared/account"
	"github.com/vecno-io/algo-collection/shared/client"
	exec "github.com/vecno-io/algo-collection/shared/execute"
)

func i32tob(val uint32) []byte {
	r := make([]byte, 4)
	for i := uint32(0); i < 4; i++ {
		r[i] = byte((val >> (8 * (3 - i))) & 0xff)
	}
	return r
}

var CreateAssetCmd = &cobra.Command{
	Use:   "createasset",
	Short: "Create an asset",
	Long:  `Creates an asset that utilizes an if from the collection.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("### Creating an asset for the collection")

		fmt.Println("--- Asset creation transaction")
		cln, err := client.MakeAlgodClient()
		if err != nil {
			fmt.Printf("failed to make algod client: %s\n", err)
			os.Exit(1)
		}

		txnParams, err := cln.SuggestedParams().Do(context.Background())
		if err != nil {
			fmt.Printf("failed to get transaction params: %s\n", err)
			os.Exit(1)
		}

		// ToDo replace these with the account loads

		ac1, err := account.LoadFromFile("./ac1.frag")
		if err != nil {
			fmt.Printf("failed to get the first account: %s\n", err)
			os.Exit(1)
		}
		ac2, err := account.LoadFromFile("./ac2.frag")
		if err != nil {
			fmt.Printf("failed to get the second account: %s\n", err)
			os.Exit(1)
		}

		// Setup the Asset Creation Transaction

		unitIdx := uint32(64511)
		unitName := fmt.Sprintf("#%d", unitIdx)
		// unitIdx := uint32(1)
		// unitName := fmt.Sprintf("#0000%d", unitIdx)
		// unitIdx := uint32(255)
		// unitName := fmt.Sprintf("#00%d", unitIdx)

		assetName := "latinum"
		assetHash := "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
		assetPath := "https://path/to/my/asset/details"

		note := []byte(nil)
		total := uint64(1)
		frozen := false
		decimals := uint32(0)

		address := ac2.Address.String()
		manager := types.ZeroAddress.String()

		freeze := types.ZeroAddress.String()
		reserve := types.ZeroAddress.String()
		clawback := types.ZeroAddress.String()

		tx1, err := future.MakeAssetCreateTxn(
			address, note, txnParams, total, decimals,
			frozen, manager, reserve, freeze, clawback,
			unitName, assetName, assetPath, assetHash,
		)
		if err != nil {
			fmt.Printf("failed to create asset creation transaction: %s\n", err)
			os.Exit(1)
		}

		// Setup the Index Reservation Transaction

		appIdx := uint64(0)
		dat, err := os.ReadFile("./app.frag")
		if err != nil {
			fmt.Printf("fail to read application file id: %s\n", err)
			os.Exit(1)
		}
		err = json.Unmarshal(dat[:], &appIdx)
		if err != nil {
			fmt.Printf("fail to decode application id: %s\n", err)
			os.Exit(1)
		}

		appArgs := [][]byte{
			[]byte("reserve"),
			i32tob(unitIdx),
		}
		accounts := []string{}
		foreignApps := []uint64{}
		foreignAssets := []uint64{}
		group := types.Digest{}
		lease := [32]byte{}
		rekeyTo := types.ZeroAddress

		// Note: Inconsitency in the api, needs a decoded address?
		sender, err := types.DecodeAddress(ac1.Address.String())
		if err != nil {
			fmt.Printf("failed to decode address: %s\n", err)
			os.Exit(1)
		}

		tx2, err := future.MakeApplicationNoOpTx(
			appIdx, appArgs, accounts, foreignApps, foreignAssets,
			txnParams, sender, note, group, lease, rekeyTo,
		)
		if err != nil {
			fmt.Printf("failed to create application call transaction: %s\n", err)
			os.Exit(1)
		}

		gid, err := crypto.ComputeGroupID([]types.Transaction{tx1, tx2})
		if err != nil {
			fmt.Printf("failed to create group id: %s\n", err)
			os.Exit(1)
		}
		tx1.Group = gid
		tx2.Group = gid

		// Note Account 1 is the contract owner, it does the contract call. The minter
		// can be annyone so the collection logic is not bount by the 1k minting limit.
		_, stx1, err := crypto.SignTransaction(ac2.PrivateKey, tx1)
		if err != nil {
			fmt.Printf("Failed to sign transaction one: %s\n", err)
			return
		}
		_, stx2, err := crypto.SignTransaction(ac1.PrivateKey, tx2)
		if err != nil {
			fmt.Printf("Failed to sign transaction two: %s\n", err)
			return
		}

		var signedGroup []byte
		signedGroup = append(signedGroup, stx1...)
		signedGroup = append(signedGroup, stx2...)

		pendingTxID, err := cln.SendRawTransaction(signedGroup).Do(context.Background())
		if err != nil {
			fmt.Printf("failed to send transaction: %s\n", err)
			os.Exit(1)
		}

		txInfo, err := client.WaitForConfirmation(cln, pendingTxID, 24, context.Background())
		if err != nil {
			fmt.Printf("failed to confirm transaction: %s\n", err)
			os.Exit(1)
		}
		if len(txInfo.PoolError) > 0 {
			fmt.Printf("error while confirm transaction: %s\n", txInfo.PoolError)
			os.Exit(1)
		}

		fmt.Printf("Asset Created deployed with id: %d\n", txInfo.AssetIndex)

		out, err := exec.List([]string{
			"-c", fmt.Sprintf("goal app read -d ./net1/primary --app-id %d --guess-format --global", appIdx),
		})
		if len(out) > 0 {
			fmt.Println()
			fmt.Println(out)
		}
		if nil != err {
			fmt.Printf("failed to read app state: %s\n", err)
			os.Exit(1)
		}
	},
}
