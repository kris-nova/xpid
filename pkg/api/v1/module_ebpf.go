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

package v1

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kris-nova/xpid/pkg/libxpid"

	"github.com/kris-nova/xpid/pkg/procfs"
)

var _ ProcessExplorerModule = &EBPFModule{}

type EBPFModule struct {
	Mounts string   `json:"mount,omitempty"`
	Progs  []string `json:"progs,omitempty"`
	Maps   []string `json:"maps,omitempty"`
}

func NewEBPFModule() *EBPFModule {
	return &EBPFModule{}
}

const (
	// Taken from <linux/bpf.h>
	// https://github.com/torvalds/linux/blob/master/include/uapi/linux/bpf.h

	FileDescriptorMapIDKey  = "map_id"
	FileDescriptorProgIDKey = "prog_id"
)

func (m *EBPFModule) Meta() *Meta {
	return &Meta{
		Name:        "eBPF module",
		Description: "Search proc(5) filesystems for eBPF programs. Will do an in depth scan and search for obfuscated directories.",
		Authors: []string{
			"Kris Nóva <kris@nivenly.com>",
		},
	}
}

func (m *EBPFModule) Execute(p *Process) error {
	// Module specific (correlated)

	procfshandle := procfs.NewProcFileSystem(procfs.Proc())
	mounts, _ := procfshandle.ContentsPID(p.PID, "mounts")
	p.Mounts = mounts

	bpfDebug, err := NewEBPFFileSystemData()
	if err != nil {
		return fmt.Errorf("unable to read /sys/fs/bpf: %v", err)
	}

	// Compare with file descriptors in fdinfo
	fds, err := procfshandle.DirPID(p.PID, "fdinfo")

	if err != nil {
		return fmt.Errorf("unable to read /proc/%d/fdinfo: %v", p.PID, err)
	}

	// File descriptor scanning
	//
	// Here we try to map the file descriptor keys (map_id, prog_id)
	// back to the established values found in the progs.debug and maps.debug
	// sys filesystem
	//
	for _, fd := range fds {
		fddata, err := procfshandle.ContentsPID(p.PID, filepath.Join("fdinfo", fd.Name()))
		if err != nil {
			continue
		}
		fdProgID := procfs.FileKeyValue(fddata, FileDescriptorProgIDKey)
		fdMapID := procfs.FileKeyValue(fddata, FileDescriptorMapIDKey)

		// Map back to /sys/fs/bpf/progs.debug
		for id, _ := range bpfDebug.Progs {
			if id == "" {
				continue
			}
			if id == fdProgID {
				// We have mapped an eBPF program to a PID!
				p.EBPF = true
				progDetails := programDetails(p, fddata)
				if progDetails != "" && !strings.Contains(strings.Join(p.EBPFModule.Progs, ""), progDetails) {
					p.EBPFModule.Progs = append(p.EBPFModule.Progs, progDetails)
				}
			}
		}

		// Map back to /sys/fs/bpf/maps.debug
		for id, _ := range bpfDebug.Maps {
			if id == "" {
				continue
			}
			if id == fdMapID {
				// We have mapped an eBPF program to a PID!
				p.EBPF = true
				mapDetails := mapDetails(p, fddata)
				if mapDetails != "" && !strings.Contains(strings.Join(p.EBPFModule.Maps, ""), mapDetails) {
					p.EBPFModule.Maps = append(p.EBPFModule.Maps, mapDetails)
				}
			}
		}
	}
	return nil
}

// EBPFFileSystemData is structured data from /sys/fs/bpf/*
type EBPFFileSystemData struct {
	Maps  map[string]*Map
	Progs map[string]*Prog
}

type Map struct {
	ID         string
	Name       string
	MaxEntries string
}
type Prog struct {
	ID       string
	Name     string
	Attached string
}

const (
	DefaultEBPFFileSystemDataDir = "/sys/fs/bpf"
)

// NewEBPFFileSystemData will read from /sys/fs/bpf/[maps.debug, progs.debug]
func NewEBPFFileSystemData() (*EBPFFileSystemData, error) {
	e := &EBPFFileSystemData{
		Progs: make(map[string]*Prog),
		Maps:  make(map[string]*Map),
	}
	mapbytes, err := ioutil.ReadFile(filepath.Join(DefaultEBPFFileSystemDataDir, "maps.debug"))
	if err != nil {
		return nil, fmt.Errorf("map read: %v", err)
	}
	mapstr := string(mapbytes)
	lines := strings.Split(mapstr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse the file
		spl := strings.Split(line, " ")
		var name, id string
		if len(spl) < 2 {
			name = ""
		} else {
			name = strings.TrimSpace(spl[1])
		}
		id = strings.TrimSpace(spl[0])

		// Ignore headers
		if id == "id" {
			continue
		}

		mp := &Map{
			ID:   id,
			Name: name,
		}
		e.Maps[id] = mp
	}

	progbytes, err := ioutil.ReadFile(filepath.Join(DefaultEBPFFileSystemDataDir, "progs.debug"))
	if err != nil {
		return nil, fmt.Errorf("prog read: %v", err)
	}
	progstr := string(progbytes)
	lines = strings.Split(progstr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse the file
		spl := strings.Split(line, " ")
		var name, id string
		if len(spl) < 2 {
			name = ""
		} else {
			name = strings.TrimSpace(spl[1])
		}
		id = strings.TrimSpace(spl[0])

		// Ignore headers
		if id == "id" {
			continue
		}
		p := &Prog{
			ID:   id,
			Name: name,
		}
		e.Progs[id] = p
	}
	return e, nil
}

// fddata is the filedescriptor data
//
// Example BPF Program File Descriptor:
//
// [root@emily]: /proc/141735/fdinfo># cat 17
// pos:    0
// flags:  02000000
// mnt_id: 15
// ino:    10586
// link_type:      perf
// link_id:        19
// prog_tag:       40bd9646d9b53ff8
// prog_id:        106
// pos:    0
//
// flags:  02000000
// mnt_id: 15
// ino:    11861
// link_type:      raw_tracepoint
// link_id:        28
// prog_tag:       1b9b934ffae90cca
// prog_id:        16097
// tp_name:        sys_enter
//
func programDetails(p *Process, fddata string) string {
	linkType := procfs.FileKeyValue(fddata, "link_type")
	progId := procfs.FileKeyValue(fddata, "prog_id")
	tpName := procfs.FileKeyValue(fddata, "tp_name")
	var progDetails string
	if tpName == "" {
		// Ignore programs with a tracepoint name
		return ""
	}
	progDetails = tpName
	if linkType != "" {
		progDetails = fmt.Sprintf("%s %s", progDetails, linkType)
	}
	if progId != "" {
		progDetails = fmt.Sprintf("%s %s", progDetails, progId)
	}
	return strings.TrimSpace(progDetails)
}

// fddata is the filedescriptor data
//
// Example BPF Map File Descriptor:
//
// [root@alice]: /proc/2582229># cat fdinfo/11
// pos:    0
// flags:  02000002
// mnt_id: 15
// ino:    11861
// map_type:       1
// key_size:       20
// value_size:     48
// max_entries:    65535
// map_flags:      0x1
// map_extra:      0x0
// memlock:        4718592
// map_id: 79
// frozen: 0
func mapDetails(p *Process, fddata string) string {
	var mapDetails string
	mapType := procfs.FileKeyValue(fddata, "map_type")
	i, err := strconv.Atoi(mapType)
	if err == nil {
		mapDetails = libxpid.BPFMapType(i)
	} else {
		mapDetails = mapType
	}
	return mapDetails
}
