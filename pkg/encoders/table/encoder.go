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

package table

import (
	"fmt"
	"github.com/fatih/color"
	encoder "github.com/kris-nova/xpid/pkg/encoders"
	"golang.org/x/term"

	filter "github.com/kris-nova/xpid/pkg/filters"

	api "github.com/kris-nova/xpid/pkg/api/v1"
)

var _ encoder.ProcessExplorerEncoder = &TableEncoder{}

type TableEncoder struct {
	filters []filter.ProcessFilter
}

func (j *TableEncoder) EncodeAll(p *api.Process) ([]byte, error) {
	return j.Encode(p)
}

func (j *TableEncoder) EncodeUser(u *api.User) ([]byte, error) {
	var str string

	// Header
	var hdr string
	hdr += fmt.Sprintf("%-*s", len(u.Name)+3, "USER")
	hdr += fmt.Sprintf("%-*s", 5, "UID")
	hdr += fmt.Sprintf("%-*s", len(u.Group.Name)+3, "GROUP")
	hdr += fmt.Sprintf("%-*s", 5, "GID")
	hdr += fmt.Sprintf("\n")
	str += drawLine()
	str += color.GreenString(hdr)

	// First line
	str += fmt.Sprintf("%-*s", len(u.Name)+3, u.Name)
	str += fmt.Sprintf("%-*d", 5, u.ID)
	str += fmt.Sprintf("%-*s", len(u.Group.Name)+3, u.Group.Name)
	str += fmt.Sprintf("%-*d", 5, u.Group.ID)
	str += fmt.Sprintf("\n")
	str += drawLine()

	return []byte(str), nil
}

var (
	TableFmtNS  bool = false
	TableFmtBPF bool = false
)

func (j *TableEncoder) Encode(p *api.Process) ([]byte, error) {
	for _, f := range j.filters {
		if !f(p) {
			return []byte(""), fmt.Errorf(filter.Filtered)
		}
	}

	var str string
	var hdr string
	if p.ShowHeader {
		// Header
		hdr += fmt.Sprintf("%-9s", "PID")
		hdr += fmt.Sprintf("%-9s", "USER")
		hdr += fmt.Sprintf("%-9s", "GROUP")
		hdr += fmt.Sprintf("%-16s", "CMD")
		if TableFmtNS {
			hdr += fmt.Sprintf("%-12s", "NS-PID")    // Compute
			hdr += fmt.Sprintf("%-12s", "NS-CGROUP") // Compute
			hdr += fmt.Sprintf("%-12s", "NS-NET")    // Network
			hdr += fmt.Sprintf("%-12s", "NS-MNT")    // Storage
		}
		if TableFmtBPF {
			hdr += fmt.Sprintf("%-16s", "BPF-MAP")
			hdr += fmt.Sprintf("%-16s", "BPF-PROG")
		}
		hdr += fmt.Sprintf("\n")
		hdrColor := color.New(color.FgGreen)
		hdr = hdrColor.Sprintf(hdr)
		str += hdr
	}

	// Lines
	x := 0
	str += color.YellowString(fmt.Sprintf("%-9d", p.PID))
	str += fmt.Sprintf("%-9s", p.User.Name)
	str += fmt.Sprintf("%-9s", p.User.Group.Name)
	str += color.CyanString(fmt.Sprintf("%-16s", p.ProcModule.Comm))
	x = x + 34
	if TableFmtNS {
		str += fmt.Sprintf("%-12s", p.NamespaceModule.PID)
		str += fmt.Sprintf("%-12s", p.NamespaceModule.Cgroup)
		str += fmt.Sprintf("%-12s", p.NamespaceModule.Net)
		str += fmt.Sprintf("%-12s", p.NamespaceModule.Mount)
		x = x + 48
	}
	if TableFmtBPF {
		var l, lm, lp int
		lm = len(p.EBPFModule.Maps)
		lp = len(p.EBPFModule.Progs)
		if lp > lm {
			l = lp
		} else {
			l = lm
		}
		for i := 0; i < l; i++ {
			fmt.Sprintf("%-*s", x, "-")
			if lm >= i+1 {
				str += fmt.Sprintf("%-16s", p.EBPFModule.Maps[i])
			} else {
				str += fmt.Sprintf("%-16s", "")
			}
			if lp >= i+1 {
				str += fmt.Sprintf("%-16s", p.EBPFModule.Progs[i])
			} else {
				str += fmt.Sprintf("%-16s", "")
			}
			str += "\n"
		}

	}
	str += fmt.Sprintf("\n")

	if p.DrawLineAfter {
		str += drawLine()
	}

	return []byte(str), nil

}

func (j *TableEncoder) AddFilter(f filter.ProcessFilter) {
	j.filters = append(j.filters, f)
}

func NewTableEncoder() *TableEncoder {
	return &TableEncoder{}
}

func drawLine() string {
	y, _, _ := term.GetSize(0)
	if y == 0 {
		return ""
	}
	lc := color.New(color.Bold, color.FgGreen)
	var str string
	for i := 0; i < y; i++ {
		str += lc.Sprintf("─")
	}
	str += "\n"
	return str
}
