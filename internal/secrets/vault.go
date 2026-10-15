package secrets

import (
	"context"
	"fmt"

	vault "github.com/hashicorp/vault/api"
)

func LoadVaultKV2(
	ctx context.Context,
	address string,
	token string,
	secretPath string,
) (map[string]string, error) {
	if address == "" {
		return nil, fmt.Errorf("Vault address is required")
	}
	if token == "" {
		return nil, fmt.Errorf("Vault token is required")
	}
	if secretPath == "" {
		return nil, fmt.Errorf("Vault secret path is required")
	}

	config := vault.DefaultConfig()
	config.Address = address
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create Vault client: %w", err)
	}
	client.SetToken(token)

	secret, err := client.KVv2("secret").Get(ctx, secretPath)
	if err != nil {
		return nil, fmt.Errorf("read Vault secret %q: %w", secretPath, err)
	}

	values := make(map[string]string, len(secret.Data))
	for key, value := range secret.Data {
		stringValue, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("Vault secret %q field %q must be a string", secretPath, key)
		}
		values[key] = stringValue
	}
	return values, nil
}
