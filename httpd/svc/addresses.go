package svc

import (
	"github.com/btcsuite/btcd/btcjson"
	"github.com/patrickmn/go-cache"
	"ledger-sats-stack/types"
	"ledger-sats-stack/utils"

	log "github.com/sirupsen/logrus"
)

func (s *Service) GetAddresses(addresses []string, blockHash *string) (types.Addresses, error) {
	// Thread-safe Bus cache, to cache result of GetTransaction from the Bus
	// against the TxID.
	//
	// cleanupInterval is set to 0 to avoid spinning up the janitor
	// goroutine.
	s.Bus.Cache = cache.New(cache.NoExpiration, 0)
	defer func() {
		if s.Bus.Cache != nil {
			s.Bus.Cache.Flush()
		}

		s.Bus.Cache = nil
	}()

	txResults, err := s.Bus.ListTransactions(blockHash)
	if err != nil {
		log.WithFields(log.Fields{
			"error":     err,
			"blockHash": nil,
		}).Error("Unable to fetch transaction")
	}
	walletTxs := s.filterTransactionsByAddresses(addresses, txResults)

	txs := []types.Transaction{}
	for _, txn := range walletTxs {
		block := blockFromTxResult(txn)
		tx, err := s.GetTransaction(txn.TxID, block)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"hash":  txn.TxID,
			}).Error("Unable to fetch transaction")

			s.Bus.Cache.Delete(txn.TxID)
			continue
		}

		// Be defensive here with the retrieved transaction, to avoid
		// nil pointer dereference.
		if tx != nil {
			txs = append(txs, *tx)
		}
	}

	return types.Addresses{
		Truncated:    false,
		Transactions: txs,
	}, nil
}

func (s *Service) filterTransactionsByAddresses(
	addresses []string, txs []btcjson.ListTransactionsResult,
) []btcjson.ListTransactionsResult {
	var result []btcjson.ListTransactionsResult
	var visited []string

	for _, tx := range txs {
		if tx.Category == "send" {
			block := blockFromTxResult(tx)
			tx2, err := s.GetTransaction(tx.TxID, block)
			if err != nil {
				log.WithFields(log.Fields{
					"error":    err,
					"hash":     tx.TxID,
					"category": tx.Category,
				}).Error("Failed to get wallet transaction")

				// abandon processing the current transaction
				continue
			}

			for _, inputAddress := range getTransactionInputAddresses(*tx2) {
				if utils.Contains(addresses, inputAddress) && !utils.Contains(visited, tx.TxID) {
					result = append(result, tx)
					visited = append(visited, tx.TxID)
					break
				}
			}
		}

		if utils.Contains(addresses, tx.Address) && !utils.Contains(visited, tx.TxID) {
			result = append(result, tx)
			visited = append(visited, tx.TxID)
		}
	}

	return result
}

func getTransactionInputAddresses(tx types.Transaction) []string {
	var result []string

	for _, txInput := range tx.Inputs {
		result = append(result, txInput.Address)
	}

	return result
}

func blockFromTxResult(tx btcjson.ListTransactionsResult) *types.Block {
	var height int64
	if tx.BlockHeight != nil {
		height = int64(*tx.BlockHeight)
	} else {
		height = -1
	}

	return &types.Block{
		Hash:   tx.BlockHash,
		Height: height,
		Time:   utils.ParseUnixTimestamp(tx.BlockTime),
	}
}
