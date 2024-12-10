package random

import (
	"math/rand/v2"
	"time"

	"github.com/goark/mt/mt19937"
)

type State struct {
	Lcg *Lcg
	Mt  *rand.Rand
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
