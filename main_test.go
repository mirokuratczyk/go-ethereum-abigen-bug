package main

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"os"
	"testing"

	"example/erc20"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// getEnv returns the value of key, or fallback if unset.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Anvil default accounts.
const deployerPrivateKeyHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

var (
	account1 = common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	account2 = common.HexToAddress("0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC")
)

// tokens converts a whole-token amount to wei (18 decimals).
func tokens(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
}

// setupClient connects to the Ethereum node and returns a client + transactor.
func setupClient(t *testing.T) (*ethclient.Client, *bind.TransactOpts) {
	t.Helper()
	rpcURL := getEnv("RPC_URL", "http://127.0.0.1:8545")

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		t.Fatalf("failed to connect to node: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		t.Fatalf("failed to fetch chain ID: %v", err)
	}

	privateKey, err := crypto.HexToECDSA(deployerPrivateKeyHex)
	if err != nil {
		t.Fatalf("invalid private key: %v", err)
	}
	ownerAddr := crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey))
	t.Logf("deployer: %s | chain ID: %s", ownerAddr.Hex(), chainID)

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		t.Fatalf("failed to create transactor: %v", err)
	}

	return client, auth
}

// deployToken deploys a fresh MyToken contract and returns the binding.
func deployToken(t *testing.T, client *ethclient.Client, auth *bind.TransactOpts) *erc20.ERC20 {
	t.Helper()

	addr, tx, token, err := erc20.DeployERC20(auth, client)
	if err != nil {
		t.Fatalf("failed to deploy contract: %v", err)
	}
	if _, err = bind.WaitMined(context.Background(), client, tx); err != nil {
		t.Fatalf("deploy tx not mined: %v", err)
	}
	t.Logf("contract deployed at: %s", addr.Hex())
	return token
}

// balanceOf reads the token balance of addr.
func balanceOf(t *testing.T, token *erc20.ERC20, addr common.Address) *big.Int {
	t.Helper()
	balance, err := token.BalanceOf(&bind.CallOpts{Context: context.Background()}, addr)
	if err != nil {
		t.Fatalf("failed to read balance of %s: %v", addr.Hex(), err)
	}
	return balance
}

// mint sends a mint transaction. If amount is negative it skips the call entirely;
// if positive it waits for mining.
func mint(t *testing.T, client *ethclient.Client, auth *bind.TransactOpts, token *erc20.ERC20, to common.Address, amount *big.Int) {
	t.Helper()
	tx, err := token.Mint(auth, to, amount)
	if err != nil {
		t.Fatalf("mint tx submission failed: %v", err)
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		t.Fatalf("mint tx not mined: %v", err)
	}
	if receipt.Status == 0 {
		t.Fatalf("mint tx reverted unexpectedly")
	}
	t.Logf("mint tx mined: block %s, gas used %d", receipt.BlockNumber, receipt.GasUsed)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestMint_ValidAmount deploys the contract, checks the initial balance is 0,
// mints 100 tokens, and asserts the balance is now 100.
func TestMint_ValidAmount(t *testing.T) {
	client, auth := setupClient(t)
	token := deployToken(t, client, auth)

	before := balanceOf(t, token, account1)
	if before.Sign() != 0 {
		t.Fatalf("expected zero initial balance, got %s", before)
	}

	mint(t, client, auth, token, account1, tokens(100))

	after := balanceOf(t, token, account1)
	t.Logf("balance after mint: %s", after)
	if after.Cmp(tokens(100)) != 0 {
		t.Errorf("expected %s, got %s", tokens(100), after)
	}
}

// TestMint_InvalidAmount deploys the contract, checks the initial balance is 0,
// attempts to mint -1 tokens, and asserts the balance is unchanged.
func TestMint_InvalidAmount(t *testing.T) {
	client, auth := setupClient(t)
	token := deployToken(t, client, auth)

	before := balanceOf(t, token, account1)
	if before.Sign() != 0 {
		t.Fatalf("expected zero initial balance, got %s", before)
	}

	mint(t, client, auth, token, account1, big.NewInt(-1))

	after := balanceOf(t, token, account1)
	t.Logf("balance after mint: %s", after)
	if after.Cmp(before) != 0 {
		t.Errorf("balance should be unchanged: expected %s, got %s", before, after)
	}
}

// TestMint_TwoTargets deploys the contract, mints 100 tokens to account1 and
// -1 tokens to account2, then asserts account1 has 100 and account2 is unchanged.
func TestMint_TwoTargets(t *testing.T) {
	client, auth := setupClient(t)
	token := deployToken(t, client, auth)

	before1 := balanceOf(t, token, account1)
	before2 := balanceOf(t, token, account2)

	mint(t, client, auth, token, account1, tokens(100))
	mint(t, client, auth, token, account2, big.NewInt(-1))

	after1 := balanceOf(t, token, account1)
	after2 := balanceOf(t, token, account2)

	t.Logf("account1 balance: %s → %s", before1, after1)
	t.Logf("account2 balance: %s → %s", before2, after2)

	if after1.Cmp(tokens(100)) != 0 {
		t.Errorf("account1: expected %s, got %s", tokens(100), after1)
	}
	if after2.Cmp(before2) != 0 {
		t.Errorf("account2: balance should be unchanged: expected %s, got %s", before2, after2)
	}
}

// TestMint_TwoTargets deploys the contract, mints 100 tokens to account1 and
// -2^255 tokens to account2, then asserts account1 has 100 and account2 is unchanged.
//
// In 2’s complement reinterpretation:
// Negative int256 values map to
// uint256 = 2^256 + x (where x < 0)
//
// That means:
// The smallest unsigned value you can get from a negative int256 is when x = -2^255
//
// That gives:
// uint256 = 2^256 - 2^255 = 2^255
//
// So all negative int256 values map into:
// [2^255, 2^256 - 1]
// Which is a range of 2^255.
//
// 2^255 = 1000...000 (1 followed by 255 zeros) is max negative int256 value
// reinterpreted as uint256.
func TestMint_TwoTargets_MinMax(t *testing.T) {
	client, auth := setupClient(t)
	token := deployToken(t, client, auth)

	before1 := balanceOf(t, token, account1)
	before2 := balanceOf(t, token, account2)

	amount := new(big.Int).Lsh(big.NewInt(1), 255) // 2^255
	amount.Neg(amount)                             // negate it

	mint(t, client, auth, token, account1, tokens(100))
	mint(t, client, auth, token, account2, amount)

	after1 := balanceOf(t, token, account1)
	after2 := balanceOf(t, token, account2)

	t.Logf("account1 balance: %s → %s", before1, after1)
	t.Logf("account2 balance: %s → %s", before2, after2)

	if after1.Cmp(tokens(100)) != 0 {
		t.Errorf("account1: expected %s, got %s", tokens(100), after1)
	}
	if after2.Cmp(before2) != 0 {
		t.Errorf("account2: balance should be unchanged: expected %s, got %s", before2, after2)
	}
}

func TestMint_TwoTargets_BothMax(t *testing.T) {
	client, auth := setupClient(t)
	token := deployToken(t, client, auth)

	before1 := balanceOf(t, token, account1)
	before2 := balanceOf(t, token, account2)

	amount := new(big.Int).Lsh(big.NewInt(1), 254) // 2^255
	amount.Neg(amount)                             // negate it

	mint(t, client, auth, token, account1, amount)
	mint(t, client, auth, token, account2, amount)

	after1 := balanceOf(t, token, account1)
	after2 := balanceOf(t, token, account2)

	t.Logf("account1 balance: %s → %s", before1, after1)
	t.Logf("account2 balance: %s → %s", before2, after2)

	if after1.Cmp(tokens(100)) != 0 {
		t.Errorf("account1: expected %s, got %s", tokens(100), after1)
	}
	if after2.Cmp(before2) != 0 {
		t.Errorf("account2: balance should be unchanged: expected %s, got %s", before2, after2)
	}
}
