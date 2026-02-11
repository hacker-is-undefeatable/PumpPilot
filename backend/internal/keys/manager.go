package keys

import (
	"encoding/hex"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Manager struct {
	ks         *keystore.KeyStore
	passphrase string
	dir        string
}

func NewManager(dir string, passphrase string) (*Manager, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("keystore dir is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	ks := keystore.NewKeyStore(dir, keystore.StandardScryptN, keystore.StandardScryptP)
	return &Manager{ks: ks, passphrase: passphrase, dir: dir}, nil
}

func (m *Manager) CreateAccount() (common.Address, error) {
	if m.passphrase == "" {
		return common.Address{}, errors.New("keystore passphrase is empty")
	}
	acct, err := m.ks.NewAccount(m.passphrase)
	if err != nil {
		return common.Address{}, err
	}
	return acct.Address, nil
}

func (m *Manager) Accounts() []common.Address {
	acctList := m.ks.Accounts()
	out := make([]common.Address, 0, len(acctList))
	for _, acct := range acctList {
		out = append(out, acct.Address)
	}
	return out
}

func (m *Manager) FindAccount(addr common.Address) (accounts.Account, error) {
	acctList := m.ks.Accounts()
	for _, acct := range acctList {
		if acct.Address == addr {
			return acct, nil
		}
	}
	return accounts.Account{}, errors.New("account not found")
}

func (m *Manager) SignTransaction(addr common.Address, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	if m.passphrase == "" {
		return nil, errors.New("keystore passphrase is empty")
	}
	acct, err := m.FindAccount(addr)
	if err != nil {
		return nil, err
	}
	return m.ks.SignTxWithPassphrase(acct, m.passphrase, tx, chainID)
}

func (m *Manager) ExportKeyJSON(addr common.Address) ([]byte, error) {
	acct, err := m.FindAccount(addr)
	if err != nil {
		return nil, err
	}
	if acct.URL.Path == "" {
		return nil, errors.New("keystore path not found")
	}
	return os.ReadFile(acct.URL.Path)
}

func (m *Manager) ExportPrivateKeyHex(addr common.Address) (string, error) {
	if m.passphrase == "" {
		return "", errors.New("keystore passphrase is empty")
	}
	acct, err := m.FindAccount(addr)
	if err != nil {
		return "", err
	}
	if acct.URL.Path == "" {
		return "", errors.New("keystore path not found")
	}
	keyJSON, err := os.ReadFile(acct.URL.Path)
	if err != nil {
		return "", err
	}
	key, err := keystore.DecryptKey(keyJSON, m.passphrase)
	if err != nil {
		return "", err
	}
	priv := key.PrivateKey
	if priv == nil {
		return "", errors.New("private key not available")
	}
	b := priv.D.Bytes()
	if len(b) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(b):], b)
		b = padded
	}
	return "0x" + hex.EncodeToString(b), nil
}

func (m *Manager) KeystoreDir() string {
	return filepath.Clean(m.dir)
}

func (m *Manager) PassphraseSet() bool {
	return m.passphrase != ""
}
