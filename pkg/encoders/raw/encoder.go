/*===========================================================================*\
 *           MIT License Copyright (c) 2022 Kris Nóva <kris@nivenly.com>     *
 *                                                                           *
 *                ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓                *
 *                ┃   ███╗   ██╗ ██████╗ ██╗   ██╗ █████╗   ┃                *
 *                ┃   ████╗  ██║██╔═████╗██║   ██║██╔══██╗  ┃                *
 *                ┃   ██╔██╗ ██║██║██╔██║██║   ██║███████║  ┃                *
 *                ┃   ██║╚██╗██║████╔╝██║╚██╗ ██╔╝██╔══██║  ┃                *
 *                ┃   ██║ ╚████║╚██████╔╝ ╚████╔╝ ██║  ██║  ┃                *
 *                ┃   ╚═╝  ╚═══╝ ╚═════╝   ╚═══╝  ╚═╝  ╚═╝  ┃                *
 *                ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛                *
 *                                                                           *
 *                       This machine kills fascists.                        *
 *                                                                           *
\*===========================================================================*/

package Raw

import (
	"fmt"

	filter "github.com/kris-nova/xpid/pkg/filters"

	api "github.com/kris-nova/xpid/pkg/api/v1"
	"github.com/kris-nova/xpid/pkg/procx"
)

var _ procx.ProcessExplorerEncoder = &RawEncoder{}

type RawEncoder struct {
	filters []filter.ProcessFilter
}

func (r *RawEncoder) Encode(p *api.Process) ([]byte, error) {
	for _, f := range r.filters {
		if !f(p) {
			return []byte(""), fmt.Errorf("filtered")
		}
	}
	str := fmt.Sprintf("name=%s pid=%d cli=%s\n", p.Name, p.PID, p.CommandLine)
	return []byte(str), nil
}

func (r *RawEncoder) AddFilter(f filter.ProcessFilter) {
	r.filters = append(r.filters, f)
}

func NewRawEncoder() *RawEncoder {
	return &RawEncoder{}
}
