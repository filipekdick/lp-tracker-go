// Package onchain reads live concentrated-liquidity pool state (current sqrt
// price, tick, active liquidity, tokens and decimals) directly from chain via
// per-chain Alchemy RPC clients, so the analyzer can ground its concentrated-APR
// projection in the REAL in-range liquidity rather than a TVL heuristic.
//
// Only canonical Uniswap-V3-style pools are supported, because they expose
// slot0() + liquidity() with the standard layout this package reads:
//
//   - uniswap_v3
//   - Aerodrome CL / Slipstream (aerodrome_cl, aerodrome-slipstream)
//   - pancakeswap_v3
//
// Everything else falls back to the analyzer's pool-data-only estimate:
//
// TODO: Algebra-based pools (Camelot v3, Thena, QuickSwap v3) expose globalState()
// instead of slot0() and are not read here.
// TODO: Uniswap v4 is a singleton (PoolManager) with no per-pool contract.
// TODO: v2 pools have no concentrated liquidity / ticks.
// TODO: band-average liquidity by walking the tick bitmap, for accuracy beyond
// the current-tick liquidity() snapshot used here.
package onchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/filipekdick/lp-tracker-go/contracts/erc20"
	"github.com/filipekdick/lp-tracker-go/contracts/pool"
)

// PoolState is the live on-chain state of a v3-style pool needed to value a
// concentrated position against the pool's real active liquidity.
type PoolState struct {
	SqrtPriceX96 *big.Int       // current sqrt price, Q64.96
	TickNow      int64          // current tick
	TickSpacing  int64          // pool tick spacing
	Liquidity    *big.Int       // active L at the current tick (uint128), raw units
	Token0       common.Address // token0 (lower address)
	Token1       common.Address // token1
	Dec0, Dec1   uint8          // token decimals
}

// Prober reads live pool state. ok=false (with nil error) means the
// pool/chain is not supported and the caller should use its fallback; a non-nil
// error is a transient read failure that should also fall back (never abort).
type Prober interface {
	PoolStateAt(ctx context.Context, chainSlug, poolAddr string) (st PoolState, ok bool, err error)
}

// NoopProber always reports "unsupported", so callers use their fallback. It is
// the zero-config Prober for tests and for running without RPC configured.
type NoopProber struct{}

func (NoopProber) PoolStateAt(context.Context, string, string) (PoolState, bool, error) {
	return PoolState{}, false, nil
}

// SupportedDEX reports whether a datasource DEX id is a canonical Uniswap-V3-style
// pool this package can read (slot0 + liquidity layout). Algebra forks (Camelot,
// Thena, QuickSwap v3 — globalState), v2 and v4 are deliberately excluded.
func SupportedDEX(dex string) bool {
	d := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(dex)), "-", "_")
	if d == "" {
		return false
	}
	// Exclude known-incompatible families even if they contain "v3".
	if strings.Contains(d, "algebra") ||
		strings.Contains(d, "camelot") ||
		strings.Contains(d, "thena") ||
		strings.Contains(d, "quickswap") ||
		strings.Contains(d, "v2") ||
		strings.Contains(d, "v4") {
		return false
	}
	switch {
	case strings.Contains(d, "uniswap") && strings.Contains(d, "v3"):
		return true
	case strings.Contains(d, "pancake") && strings.Contains(d, "v3"):
		return true
	case strings.Contains(d, "slipstream"):
		return true
	case strings.Contains(d, "aerodrome") && strings.Contains(d, "cl"):
		return true
	case strings.Contains(d, "velodrome") && strings.Contains(d, "cl"):
		return true
	default:
		return false
	}
}

type cachedState struct {
	st  PoolState
	ok  bool
	at  time.Time
	err error
}

// RPCProber reads pool state via per-chain ethclient connections. Results are
// cached per pool for a TTL (the scanner refresh cadence) so a refresh costs at
// most one batch of reads per pool; token decimals are cached forever.
type RPCProber struct {
	clients map[string]*ethclient.Client
	ttl     time.Duration

	mu         sync.Mutex
	stateCache map[string]cachedState
	decCache   map[common.Address]uint8
}

// NewRPCProber builds a prober from a chain-slug → ethclient map. ttl bounds how
// long a pool's state is cached (use the scanner refresh interval). A nil/empty
// map yields a prober that reports every pool unsupported (like NoopProber).
func NewRPCProber(clients map[string]*ethclient.Client, ttl time.Duration) *RPCProber {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &RPCProber{
		clients:    clients,
		ttl:        ttl,
		stateCache: map[string]cachedState{},
		decCache:   map[common.Address]uint8{},
	}
}

// Chains returns the configured chain slugs (for logging).
func (p *RPCProber) Chains() []string {
	out := make([]string, 0, len(p.clients))
	for slug := range p.clients {
		out = append(out, slug)
	}
	return out
}

// PoolStateAt reads (or serves from cache) the live state of a v3-style pool.
func (p *RPCProber) PoolStateAt(ctx context.Context, chainSlug, poolAddr string) (PoolState, bool, error) {
	client, has := p.clients[chainSlug]
	if !has || client == nil {
		return PoolState{}, false, nil
	}

	key := chainSlug + "|" + strings.ToLower(poolAddr)
	p.mu.Lock()
	if c, ok := p.stateCache[key]; ok && time.Since(c.at) < p.ttl {
		p.mu.Unlock()
		return c.st, c.ok, c.err
	}
	p.mu.Unlock()

	st, ok, err := p.read(ctx, client, poolAddr)

	p.mu.Lock()
	p.stateCache[key] = cachedState{st: st, ok: ok, at: time.Now(), err: err}
	p.mu.Unlock()
	return st, ok, err
}

// read performs the actual on-chain reads for one pool.
func (p *RPCProber) read(ctx context.Context, client *ethclient.Client, poolAddr string) (PoolState, bool, error) {
	addr := common.HexToAddress(poolAddr)
	pc, err := pool.NewPool(addr, client)
	if err != nil {
		return PoolState{}, false, fmt.Errorf("binding pool %s: %w", poolAddr, err)
	}
	opts := &bind.CallOpts{Context: ctx}

	slot0, err := pc.Slot0(opts)
	if err != nil {
		return PoolState{}, false, fmt.Errorf("slot0 %s: %w", poolAddr, err)
	}
	liquidity, err := pc.Liquidity(opts)
	if err != nil {
		return PoolState{}, false, fmt.Errorf("liquidity %s: %w", poolAddr, err)
	}
	spacing, err := pc.TickSpacing(opts)
	if err != nil {
		return PoolState{}, false, fmt.Errorf("tickSpacing %s: %w", poolAddr, err)
	}
	token0, err := pc.Token0(opts)
	if err != nil {
		return PoolState{}, false, fmt.Errorf("token0 %s: %w", poolAddr, err)
	}
	token1, err := pc.Token1(opts)
	if err != nil {
		return PoolState{}, false, fmt.Errorf("token1 %s: %w", poolAddr, err)
	}
	dec0, err := p.decimals(ctx, client, token0)
	if err != nil {
		return PoolState{}, false, fmt.Errorf("decimals token0 %s: %w", token0.Hex(), err)
	}
	dec1, err := p.decimals(ctx, client, token1)
	if err != nil {
		return PoolState{}, false, fmt.Errorf("decimals token1 %s: %w", token1.Hex(), err)
	}

	return PoolState{
		SqrtPriceX96: slot0.SqrtPriceX96,
		TickNow:      slot0.Tick.Int64(),
		TickSpacing:  spacing.Int64(),
		Liquidity:    liquidity,
		Token0:       token0,
		Token1:       token1,
		Dec0:         dec0,
		Dec1:         dec1,
	}, true, nil
}

// decimals reads (and forever-caches) an ERC20's decimals.
func (p *RPCProber) decimals(ctx context.Context, client *ethclient.Client, token common.Address) (uint8, error) {
	p.mu.Lock()
	if d, ok := p.decCache[token]; ok {
		p.mu.Unlock()
		return d, nil
	}
	p.mu.Unlock()

	erc, err := erc20.NewERC20(token, client)
	if err != nil {
		return 0, err
	}
	d, err := erc.Decimals(&bind.CallOpts{Context: ctx})
	if err != nil {
		return 0, err
	}
	p.mu.Lock()
	p.decCache[token] = d
	p.mu.Unlock()
	return d, nil
}
