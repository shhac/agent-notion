package credential

import (
	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/lib-agent-cli/creds"
)

const v3TokenAccount = "v3:token_v2"

// KeychainWriter stores secrets. *keyring.Keyring satisfies it.
type KeychainWriter interface {
	Available() bool
	Set(account, secret string) error
}

// DefaultKeychainWriter writes to the same keychain service the TS CLI used.
func DefaultKeychainWriter() KeychainWriter { return creds.NewKeychain(config.KeychainService) }

// StoreV3Session persists a desktop/browser token_v2 session: the token goes to
// the keychain when available (config holds the placeholder), otherwise into
// config.json. It returns "keychain" or "config".
func StoreV3Session(sess config.V3Session, kc KeychainWriter) (string, error) {
	cfg := config.Read()

	storage := "config"
	if kc != nil && kc.Available() {
		if err := kc.Set(v3TokenAccount, sess.TokenV2); err == nil {
			sess.TokenV2 = config.KeychainPlaceholder
			storage = "keychain"
		}
	}

	cfg.V3 = &sess
	if err := config.Write(cfg); err != nil {
		return "", err
	}
	return storage, nil
}

// ResolveV3Token returns the stored token_v2, reading the keychain when config
// holds the placeholder.
func ResolveV3Token(cfg config.Config, kc Keychain) (string, bool) {
	if cfg.V3 == nil || cfg.V3.TokenV2 == "" {
		return "", false
	}
	if cfg.V3.TokenV2 == config.KeychainPlaceholder {
		return kc.Get(v3TokenAccount)
	}
	return cfg.V3.TokenV2, true
}
