package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func (s *Server) handleZeroGTxStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	txHash := strings.TrimSpace(r.URL.Query().Get("txHash"))
	if txHash == "" {
		writeError(w, http.StatusBadRequest, "missing txHash")
		return
	}
	if !common.IsHexHash(txHash) {
		writeError(w, http.StatusBadRequest, "invalid txHash")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	mined, success, blockNumber, err := txReceiptStatus(ctx, txHash)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	status := "pending"
	if mined && success {
		status = "success"
	} else if mined && !success {
		status = "failed"
	}

	explorer := strings.TrimRight(strings.TrimSpace(os.Getenv("ZERO_G_EXPLORER_BASE")), "/")
	if explorer == "" {
		explorer = "https://chainscan-galileo.0g.ai"
	}
	explorerTx := ""
	if explorer != "" {
		explorerTx = explorer + "/tx/" + txHash
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"txHash":        txHash,
		"mined":         mined,
		"success":       success,
		"status":        status,
		"blockNumber":   blockNumber,
		"explorerTxUrl": explorerTx,
	})
}

func txReceiptStatus(ctx context.Context, txHash string) (mined bool, success bool, blockNumber uint64, err error) {
	evmRPC := strings.TrimSpace(os.Getenv("ZERO_G_EVM_RPC"))
	if evmRPC == "" {
		evmRPC = "https://evmrpc-testnet.0g.ai"
	}
	ec, err := ethclient.DialContext(ctx, evmRPC)
	if err != nil {
		return false, false, 0, err
	}
	defer ec.Close()

	h := common.HexToHash(txHash)
	receipt, rerr := ec.TransactionReceipt(ctx, h)
	if rerr == nil && receipt != nil {
		return true, receipt.Status == 1, receipt.BlockNumber.Uint64(), nil
	}
	if errors.Is(rerr, ethereum.NotFound) || strings.Contains(strings.ToLower(rerr.Error()), "not found") {
		return false, false, 0, nil
	}
	if rerr != nil {
		return false, false, 0, rerr
	}
	return false, false, 0, nil
}

