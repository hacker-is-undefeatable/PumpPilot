package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"pumppilot/internal/config"
	"pumppilot/internal/keys"
	"pumppilot/internal/trade"
	"pumppilot/internal/txbuilder"
)

type Server struct {
	cfg       *config.Config
	logger    *slog.Logger
	keys      *keys.Manager
	trade     *trade.Service
	rpcClient *rpc.Client
	ethClient *ethclient.Client
}

func NewServer(cfg *config.Config, logger *slog.Logger, keys *keys.Manager, tradeSvc *trade.Service, rpcClient *rpc.Client, ethClient *ethclient.Client) *Server {
	return &Server{cfg: cfg, logger: logger, keys: keys, trade: tradeSvc, rpcClient: rpcClient, ethClient: ethClient}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.withAuth(s.handleHealth))
	mux.HandleFunc("/keys", s.withAuth(s.handleKeys))
	mux.HandleFunc("/keys/export", s.withAuth(s.handleKeyExport))
	mux.HandleFunc("/balances", s.withAuth(s.handleBalances))
	mux.HandleFunc("/trade/buy", s.withAuth(s.handleBuy))
	mux.HandleFunc("/trade/sell", s.withAuth(s.handleSell))
	mux.HandleFunc("/trade/approve", s.withAuth(s.handleApprove))
	mux.HandleFunc("/trade/transfer", s.withAuth(s.handleTransfer))
	return mux
}

func (s *Server) Start(ctx context.Context) error {
	server := &http.Server{
		Addr:              s.cfg.API.Listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctxTimeout)
	}()
	return server.ListenAndServe()
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.API.AuthToken != "" {
			token := r.Header.Get("X-API-Key")
			if token == "" {
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
					token = strings.TrimSpace(auth[7:])
				}
			}
			if token != s.cfg.API.AuthToken {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		addrs := s.keys.Accounts()
		out := make([]string, 0, len(addrs))
		for _, a := range addrs {
			out = append(out, a.Hex())
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"keys": out})
	case http.MethodPost:
		addr, err := s.keys.CreateAccount()
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"address": addr.Hex()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type exportRequest struct {
	Address string `json:"address"`
	Format  string `json:"format"` // "keystore" or "private"
}

func (s *Server) handleKeyExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req exportRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	addr, err := parseAddress(req.Address)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "keystore"
	}
	if format == "private" {
		if !s.cfg.KeyStore.AllowPrivateExport {
			writeError(w, http.StatusForbidden, "private export disabled")
			return
		}
		keyHex, err := s.keys.ExportPrivateKeyHex(addr)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"address": addr.Hex(), "private_key": keyHex})
		return
	}
	data, err := s.keys.ExportKeyJSON(addr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"address": addr.Hex(), "keystore": string(data)})
}

func (s *Server) handleBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	addrStr := r.URL.Query().Get("address")
	if addrStr == "" {
		writeError(w, http.StatusBadRequest, "address is required")
		return
	}
	addr, err := parseAddress(addrStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		bal, err := s.ethClient.BalanceAt(r.Context(), addr, nil)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"address": addr.Hex(), "eth_wei": bal.String()})
		return
	}
	tokenAddr, err := parseAddress(token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bal, err := txbuilder.ReadERC20Balance(r.Context(), s.rpcClient, tokenAddr, addr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	decimals, err := txbuilder.ReadERC20Decimals(r.Context(), s.rpcClient, tokenAddr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"address":     addr.Hex(),
		"token":       tokenAddr.Hex(),
		"balance_wei": bal.String(),
		"decimals":    decimals,
	})
}

func (s *Server) handleBuy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req trade.BuyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.trade.Buy(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleSell(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req trade.SellRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.trade.Sell(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req trade.ApproveRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.trade.Approve(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req trade.TransferRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.trade.Transfer(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func readJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return errors.New("empty body")
	}
	return json.Unmarshal(b, v)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func parseAddress(value string) (common.Address, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return common.Address{}, errors.New("address is required")
	}
	if !common.IsHexAddress(value) {
		return common.Address{}, errors.New("invalid address")
	}
	return common.HexToAddress(value), nil
}
