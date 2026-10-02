package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageConfiguration(t *testing.T) {
	for _, kind := range []string{"fs", "filesystem", "FILESYSTEM"} {
		t.Run(kind, func(t *testing.T) {
			cmd := &cobra.Command{}
			AddStorageFlags(cmd)
			require.NoError(t, cmd.Flags().Set(FlagStorageType, kind))
			require.NoError(t, cmd.Flags().Set(FlagFsPath, t.TempDir()))
			storage, err := GetStorage(cmd)
			require.NoError(t, err)
			require.NotNil(t, storage)
		})
	}
	cmd := &cobra.Command{}
	AddStorageFlags(cmd)
	require.NoError(t, cmd.Flags().Set(FlagStorageType, "typo"))
	_, err := GetStorage(cmd)
	require.ErrorContains(t, err, "unsupported storage type")
}

func TestServerListenerPolicy(t *testing.T) {
	for _, tc := range []struct {
		address, token string
		allow, valid   bool
	}{
		{"127.0.0.1:8085", "", false, true},
		{"[::1]:8085", "", false, true},
		{"0.0.0.0:8085", "", false, false},
		{":8085", "", false, false},
		{"[::]:8085", "", false, false},
		{"192.168.1.1:8085", "", false, false},
		{"localhost:8085", "", false, false},
		{"0.0.0.0:8085", "secret", false, true},
		{"0.0.0.0:8085", "", true, true},
		{"0.0.0.0:8085", "  ", false, false},
		{"invalid", "secret", false, false},
	} {
		t.Run(tc.address+tc.token, func(t *testing.T) {
			err := validateServerListener(tc.address, tc.token, tc.allow)
			assert.Equal(t, tc.valid, err == nil)
		})
	}
}

func TestPublicListenerOptInPrecedence(t *testing.T) {
	defer viper.Reset()
	viper.Reset()
	cmd := &cobra.Command{}
	cmd.Flags().Bool(FlagAllowUnauthenticatedPublic, false, "")
	bindEnv(FlagAllowUnauthenticatedPublic, "ALLOW_UNAUTHENTICATED_PUBLIC")
	t.Setenv("ALLOW_UNAUTHENTICATED_PUBLIC", "true")
	assert.True(t, GetBool(cmd, FlagAllowUnauthenticatedPublic))
	require.NoError(t, cmd.Flags().Set(FlagAllowUnauthenticatedPublic, "false"))
	assert.False(t, GetBool(cmd, FlagAllowUnauthenticatedPublic))
}

func TestInitializeIndexerRetryAndCancellation(t *testing.T) {
	transient := errors.New("dependency unavailable")
	attempts := 0
	expected := &indexer.Indexer{}
	idx, err := initializeIndexer(context.Background(), 3, 0, func() (*indexer.Indexer, error) {
		attempts++
		if attempts == 1 {
			return nil, transient
		}
		return expected, nil
	})
	require.NoError(t, err)
	assert.Same(t, expected, idx)
	assert.Equal(t, 2, attempts)

	attempts = 0
	_, err = initializeIndexer(context.Background(), 3, 0, func() (*indexer.Indexer, error) {
		attempts++
		return nil, transient
	})
	require.ErrorIs(t, err, transient)
	assert.Equal(t, 3, attempts)

	ctx, cancel := context.WithCancel(context.Background())
	attempts = 0
	_, err = initializeIndexer(ctx, 3, time.Hour, func() (*indexer.Indexer, error) {
		attempts++
		cancel()
		return nil, transient
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, attempts)
}

func TestInvalidLLMConfigurationFails(t *testing.T) {
	cmd := &cobra.Command{}
	AddLLMFlags(cmd)
	require.NoError(t, cmd.Flags().Set(FlagLLMAPIAddr, "file:///not-a-service"))
	client, err := BuildLLMClient(cmd)
	require.Error(t, err)
	assert.Nil(t, client)
}
