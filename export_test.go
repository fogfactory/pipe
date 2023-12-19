package pipe

import "github.com/panjf2000/ants/v2"

// Pools returns the underlying pools
func (p *Pools) Pools() []*ants.Pool {
	if p == nil {
		return nil
	}
	return p.pools
}
