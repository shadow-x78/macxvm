// MacXVM - VFIO Leases (MIT)
// https://github.com/shadow-x78/macxvm

package vfio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shadow-x78/macxvm/internal/paths"
)

const (
	LeaseExclusive = syscall.LOCK_EX
	LeaseShared    = syscall.LOCK_SH
)

type LeaseOwner struct {
	VMName    string   `json:"vmName,omitempty"`
	VMID      string   `json:"vmId,omitempty"`
	Backend   string   `json:"backend,omitempty"`
	PID       int      `json:"pid,omitempty"`
	StartTime string   `json:"startTime,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	Acquired  string   `json:"acquired,omitempty"`
}

type Lease struct {
	Key  string
	Path string
	file *os.File
}

type LeaseSet struct {
	leases []*Lease
	files  []*os.File
}

func LeaseDir() string {
	return filepath.Join(paths.Run(), "leases")
}

func leasePath(key string) string {
	return filepath.Join(LeaseDir(), sanitizeKey(key)+".lease")
}

func sanitizeKey(key string) string {
	var b strings.Builder
	for _, c := range key {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '-', c == '_':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func LeaseKeysFor(cfgAddresses []string, groups []int, usbControllers []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(key string) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, key)
	}
	for _, address := range cfgAddresses {
		add("pci:" + address)
	}
	for _, group := range groups {
		add("iommu:" + strconv.Itoa(group))
	}
	for _, controller := range usbControllers {
		add("usb:" + controller)
	}
	sort.Strings(out)
	return out
}

func Acquire(owner LeaseOwner, keys []string, mode int) (*LeaseSet, error) {
	if len(keys) == 0 {
		return &LeaseSet{}, nil
	}
	if err := os.MkdirAll(LeaseDir(), 0o700); err != nil {
		return nil, err
	}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	set := &LeaseSet{}
	for _, key := range sorted {
		lease, err := acquireOne(owner, key, mode)
		if err != nil {
			set.Close()
			return nil, err
		}
		set.leases = append(set.leases, lease)
		set.files = append(set.files, lease.file)
	}
	return set, nil
}

func acquireOne(owner LeaseOwner, key string, mode int) (*Lease, error) {
	path := leasePath(key)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	flags := syscall.LOCK_NB
	if mode == LeaseShared {
		flags |= syscall.LOCK_SH
	} else {
		flags |= syscall.LOCK_EX
	}
	for attempt := 0; attempt < 2; attempt++ {
		err = syscall.Flock(int(f.Fd()), flags)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK {
			f.Close()
			return nil, fmt.Errorf("cannot lock the lease %s: %w", key, err)
		}
		if !staleLease(f, key) {
			f.Close()
			return nil, fmt.Errorf("the lease %s is already held by another virtual machine", key)
		}
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("cannot lock the lease %s: %w", key, err)
	}
	lease := &Lease{Key: key, Path: path, file: f}
	if err := lease.writeOwner(owner); err != nil {
		lease.Release()
		return nil, err
	}
	return lease, nil
}

func (l *Lease) writeOwner(owner LeaseOwner) error {
	if owner.PID == 0 {
		owner.PID = os.Getpid()
	}
	if owner.Acquired == "" {
		owner.Acquired = time.Now().UTC().Format(time.RFC3339Nano)
	}
	raw, err := json.MarshalIndent(owner, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := l.file.Truncate(0); err != nil {
		return err
	}
	if _, err := l.file.WriteAt(raw, 0); err != nil {
		return err
	}
	return l.file.Sync()
}

func (l *Lease) Release() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
	l.file = nil
}

func (s *LeaseSet) Files() []*os.File {
	if s == nil {
		return nil
	}
	return s.files
}

func (s *LeaseSet) SetOwnerPID(pid int) error {
	if s == nil || pid <= 0 {
		return nil
	}
	start, _ := ProcessStartTime(pid)
	for _, lease := range s.leases {
		if lease.file == nil {
			continue
		}
		raw, err := os.ReadFile(lease.Path)
		if err != nil {
			continue
		}
		var owner LeaseOwner
		if json.Unmarshal(raw, &owner) != nil {
			continue
		}
		owner.PID = pid
		owner.StartTime = start
		owner.Acquired = time.Now().UTC().Format(time.RFC3339Nano)
		if err := lease.writeOwner(owner); err != nil {
			return err
		}
	}
	return nil
}

func Holders(keys []string) []LeaseOwner {
	var out []LeaseOwner
	for _, key := range keys {
		owner, held := ReadOwner(key)
		if !held {
			continue
		}
		if owner.PID > 0 && !ProcessAlive(owner.PID) {
			continue
		}
		out = append(out, owner)
	}
	return out
}

func (s *LeaseSet) Keys() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.leases))
	for _, lease := range s.leases {
		out = append(out, lease.Key)
	}
	return out
}

func (s *LeaseSet) Close() {
	if s == nil {
		return
	}
	for _, lease := range s.leases {
		lease.Release()
	}
	s.leases = nil
	s.files = nil
}

func ReadOwner(key string) (LeaseOwner, bool) {
	raw, err := os.ReadFile(leasePath(key))
	if err != nil {
		return LeaseOwner{}, false
	}
	var owner LeaseOwner
	if err := json.Unmarshal(raw, &owner); err != nil {
		return LeaseOwner{}, false
	}
	return owner, true
}

func staleLease(f *os.File, key string) bool {
	raw, err := os.ReadFile(f.Name())
	if err != nil {
		return false
	}
	var owner LeaseOwner
	if err := json.Unmarshal(raw, &owner); err != nil {
		return false
	}
	if owner.PID <= 0 {
		return false
	}
	if !ProcessAlive(owner.PID) {
		return true
	}
	start, err := ProcessStartTime(owner.PID)
	if err != nil {
		return true
	}
	if owner.StartTime != "" && owner.StartTime != start {
		return true
	}
	return false
}

func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	idx := strings.LastIndex(string(raw), ")")
	if idx < 0 || idx+2 >= len(raw) {
		return false
	}
	switch raw[idx+2] {
	case 'Z', 'X', 'x':
		return false
	}
	return true
}

func ProcessStartTime(pid int) (string, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	idx := strings.LastIndex(string(raw), ")")
	if idx < 0 {
		return "", fmt.Errorf("the process %d has no valid stat line", pid)
	}
	fields := strings.Fields(string(raw[idx+2:]))
	if len(fields) < 20 {
		return "", fmt.Errorf("the process %d has no start time", pid)
	}
	return fields[19], nil
}

func CurrentOwner(owner LeaseOwner) LeaseOwner {
	if owner.PID == 0 {
		owner.PID = os.Getpid()
	}
	if start, err := ProcessStartTime(owner.PID); err == nil {
		owner.StartTime = start
	}
	if owner.Acquired == "" {
		owner.Acquired = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return owner
}
