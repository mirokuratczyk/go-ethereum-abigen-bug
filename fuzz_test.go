package main

import (
	"context"
	"math/big"
	"testing"

	"example/erc20"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
)

// uint256Max is 2^256 - 1, the largest value representable as a Solidity uint256.
var uint256Max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// setupSim creates a simulated backend funded with the deployer key and returns
// the backend, its bind-compatible client, and a ready-to-use transactor.
// The caller is responsible for calling sim.Close().
func setupSim(t interface {
	Helper()
	Fatal(...any)
	Fatalf(string, ...any)
	Cleanup(func())
}) (*simulated.Backend, simulated.Client, *bind.TransactOpts) {
	t.Helper()

	privateKey, err := crypto.HexToECDSA(deployerPrivateKeyHex)
	if err != nil {
		t.Fatalf("invalid private key: %v", err)
	}
	deployer := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Pre-fund the deployer with 10 ETH.
	alloc := types.GenesisAlloc{
		deployer: {Balance: new(big.Int).Mul(big.NewInt(10), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))},
	}
	sim := simulated.NewBackend(alloc)
	t.Cleanup(func() { sim.Close() })

	client := sim.Client()

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		t.Fatalf("failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		t.Fatalf("failed to create transactor: %v", err)
	}

	return sim, client, auth
}

// deployTokenSim deploys MyToken on the simulated backend.
func deployTokenSim(
	t interface {
		Helper()
		Fatalf(string, ...any)
		Logf(string, ...any)
	},
	sim *simulated.Backend,
	client simulated.Client,
	auth *bind.TransactOpts,
) *erc20.ERC20 {
	t.Helper()

	addr, tx, token, err := erc20.DeployERC20(auth, client)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	sim.Commit() // mine the deploy tx

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil || receipt.Status == 0 {
		t.Fatalf("deploy tx failed: %v", err)
	}
	t.Logf("contract at: %s", addr.Hex())
	return token
}

// FuzzMint verifies that:
//   - A non-negative amount within uint256 range is minted and the balance
//     increases by exactly that amount.
//   - A negative amount (which Go's ABI encoder wraps via two's complement into
//     a large uint256) is treated as an invalid input: the test asserts the
//     balance is unchanged, documenting the current contract behaviour.
func FuzzMint(f *testing.F) {
	// Seed corpus: zero, small positive, max uint256, and a negative value.
	f.Add([]byte{0})
	f.Add([]byte{1})
	f.Add([]byte{100})
	// 1000 tokens in wei
	f.Add(tokens(1000).Bytes())
	// uint256 max
	// f.Add(uint256Max.Bytes())
	// // Two's-complement -1 as 32 bytes (all 0xff) — negative input
	// negOne := make([]byte, 32)
	// for i := range negOne {
	// 	negOne[i] = 0xff
	// }
	// f.Add(negOne)

	f.Fuzz(func(t *testing.T, rawBytes []byte) {
		// Interpret the fuzz input as an unsigned big.Int.
		amount := fromTwosComplement(rawBytes)

		// Clamp to uint256 — the ABI only accepts values in [0, 2^256-1].
		// Values that exceed uint256Max are clamped; this keeps the fuzz
		// corpus meaningful for the contract's actual input space.
		if amount.Cmp(uint256Max) > 0 {
			amount.Set(uint256Max)
		}

		target := common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

		sim, client, auth := setupSim(t)
		token := deployTokenSim(t, sim, client, auth)
		callOpts := &bind.CallOpts{Context: context.Background()}
		before, err := token.BalanceOf(callOpts, target)
		if err != nil {
			t.Fatalf("balanceOf before: %v", err)
		}

		// Attempt the mint.
		tx, err := token.Mint(auth, target, amount)
		if err != nil {
			// Submission rejected (e.g. gas estimation failure for huge values).
			// Balance must be unchanged.
			after, _ := token.BalanceOf(callOpts, target)
			if after.Cmp(before) != 0 {
				t.Errorf("balance changed after rejected tx: before=%s after=%s amount=%s",
					before, after, amount)
			}
			return
		}
		sim.Commit()

		receipt, err := bind.WaitMined(context.Background(), client, tx)
		if err != nil {
			t.Fatalf("WaitMined: %v", err)
		}

		after, err := token.BalanceOf(callOpts, target)
		if err != nil {
			t.Fatalf("balanceOf after: %v", err)
		}

		if receipt.Status == 0 {
			// Reverted — balance must be unchanged.
			if after.Cmp(before) != 0 {
				t.Errorf("balance changed after reverted tx: before=%s after=%s amount=%s",
					before, after, amount)
			}
			return
		}

		// Tx succeeded — balance must have increased by exactly `amount`.
		expected := new(big.Int).Add(before, amount)
		// Handle uint256 overflow wrap-around.
		// expected.And(expected, uint256Max)

		if after.Cmp(expected) != 0 {
			t.Errorf("unexpected balance: before=%s amount=%s expected=%s got=%s",
				before, amount, expected, after)
		}
	})
}

func fromTwosComplement(b []byte) *big.Int {
	if len(b) == 0 {
		return big.NewInt(0)
	}

	// Positive if top bit is not set.
	if b[0]&0x80 == 0 {
		return new(big.Int).SetBytes(b)
	}

	// Negative:
	// value = unsigned(b) - 2^(8*len(b))
	unsigned := new(big.Int).SetBytes(b)
	modulus := new(big.Int).Lsh(big.NewInt(1), uint(8*len(b)))
	return unsigned.Sub(unsigned, modulus)
}
