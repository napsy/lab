package ws

import (
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"sync"
)

const (
	FlagDir uint64 = 1 << iota
	FlagLogical
	FlagMount
	FlagIgnore
)

type Id uint32

func NewId(path string) Id {
	h := fnv.New32()
	h.Write([]byte(path))
	return Id(h.Sum32())
}

type Dir struct {
	Path     string
	Children []*Res
}

type Res struct {
	sync.Mutex
	Id     Id
	Name   string
	Flag   uint64
	Parent *Res
	*Dir
}

func (r *Res) path(lock bool) string {
	if r == nil {
		return ""
	}
	if lock {
		r.Lock()
		defer r.Unlock()
	}
	if r.Dir != nil {
		return r.Dir.Path
	}
	p := r.Parent.path(lock)
	if len(p) < 2 {
		return p + r.Name
	}
	return p + string(os.PathSeparator) + r.Name
}

func (r *Res) Path() string {
	return r.path(true)
}

func newChild(pa *Res, name string, isdir, stat bool) (*Res, error) {
	r := &Res{Name: name, Parent: pa}
	path := r.path(false)
	r.Id = NewId(path)
	if stat {
		fi, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		isdir = fi.IsDir()
	}
	if isdir {
		r.Flag |= FlagDir
		r.Dir = &Dir{Path: path}
	}
	return r, nil
}

type byTypeAndName []*Res

func (l byTypeAndName) Len() int {
	return len(l)
}
func less(i, j *Res) bool {
	if isdir := i.Flag&FlagDir != 0; isdir != (j.Flag&FlagDir != 0) {
		return isdir
	}
	return i.Name < j.Name
}
func (l byTypeAndName) Less(i, j int) bool {
	return less(l[i], l[j])
}
func (l byTypeAndName) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func insert(l []*Res, r *Res) []*Res {
	i := sort.Search(len(l), func(i int) bool {
		return less(r, l[i])
	})
	if i < len(l) {
		if i > 0 && l[i-1].Id == r.Id {
			l[i-1] = r
			return l
		}
		return append(l[:i], append([]*Res{r}, l[i:]...)...)
	}
	return append(l, r)
}
func remove(l []*Res, r *Res) []*Res {
	i := sort.Search(len(l), func(i int) bool {
		return less(r, l[i])
	})
	if i > 0 && l[i-1].Id == r.Id {
		return append(l[:i-1], l[i:]...)
	}
	return l
}
func find(l []*Res, name string) *Res {
	for _, r := range l {
		if r.Name == name {
			return r
		}
	}
	return nil
}

var Skip = fmt.Errorf("skip")

func walk(l []*Res, f func(*Res) error) error {
	var err error
	for _, c := range l {
		if err = f(c); err == Skip {
			continue
		}
		if err != nil {
			return err
		}
		var cl []*Res
		c.Lock()
		if c.Dir != nil {
			cl = c.Children
		}
		c.Unlock()
		if len(cl) > 0 {
			if err = walk(c.Children, f); err != nil {
				return err
			}
		}
	}
	return nil
}