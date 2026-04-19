package random

import (
	mathrand "math/rand"
	"math/rand/v2"
	"time"

	"github.com/goark/mt/mt19937"
)

// State holds all per-request PRNGs. Each field is an independent MT19937
// (or other) instance; seeding one does not disturb the others. mt_rand /
// rand use Mt; gmp_random_* uses Gmp.
type State struct {
	Lcg *Lcg
	Mt  *rand.Rand

	// Gmp is the seeded source for gmp_random_* functions. It is nil
	// until gmp_random_seed() is called, at which point gmp falls back
	// to crypto/rand (matching PHP's libgmp behavior when no seed has
	// been set). It uses math/rand v1 because math/big's Rand method
	// takes a v1 *rand.Rand.
	Gmp *mathrand.Rand
}

func New() *State {
	return &State{
		Lcg: NewLcg(),
		Mt:  rand.New(mt19937.New(time.Now().UnixMicro())),
	}
}

func (s *State) MtSeed(seed int64) {
	s.Mt = rand.New(mt19937.New(seed))
}

// GmpSeed seeds the gmp_random_* source with a dedicated MT19937 state.
// This state is independent of Mt, so calling gmp_random_seed() does not
// disturb mt_rand()'s sequence.
func (s *State) GmpSeed(seed int64) {
	s.Gmp = mathrand.New(mt19937.New(seed))
}
