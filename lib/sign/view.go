package sign

import (
	"sort"
	"sync"

	"github.com/dedis/cothority/lib/dbg"
)

// View represents  the set on what to run a cothority /or another protocol)
// It is higher level than the lib/tree and peerlist
type View struct {
	sync.RWMutex
	// The number of the view (XXX still relevant ? since we have tree.id() +
	// peeers.Id()
	Num int

	tree.Node
	tree.PeerList
}

func (v *View) Id() hashid.HashId {
	return append(v.Node.Id(), v.PeerList.Id()...)
}

// Generate a new view from this Views struct
func (v *Views) NewView(view int, node tree.Node, peerList tree.PeerList) {
	dbg.Lvl3("Creating new view", view, " with", len(peerList.Peers), " hosts")
	v.Lock()
	peers := peerList.Copy()
	vi := &View{Num: view, Node: node, PeerList: peers}
	v.Views[view] = vi
	v.Unlock()
}

// XXX The following function are commented because I think they might be
// irrelevant with our new architecture (Tree + PeerList). A View is basically a
// Tree + PeerList, this pair is supposed to be unique. If we want to remove a a
// node in the tree we make a NewView out of this new tree and it will have a
// new Unique Id. We should not change the inners of a view...
/*// Remove a child from this view. Returns true if succeeded otherwise false*/
//func (v *View) RemoveChild(child string) bool {
//v.Lock()
//defer v.Unlock()

//if !v.Node.ParentOf(child) {
//return false
//}
//var i int
//for i, c := range v.Node.Children {
//if c.Name() == child {
//break
//}
//}

//v.Node.Children = append(v.Node.Children[:i], v.Node.Children[i+1:])
//// XXX should we remove also from the peer list ?? I think not because the
//// peer list is more constant within a view than a the actual tree (quickly
//// remove one malicious peer or whatever but keep the same peer list)
//return true
//}

//func (v *View) AddPeerToHostlist(name string) {
//m := make(map[string]bool)
//for _, h := range v.HostList {
//if h != name {
//m[h] = true
//}
//}
//m[name] = true
//hostlist := make([]string, 0, len(m))

//for h := range m {
//hostlist = append(hostlist, h)
//}

//sortable := sort.StringSlice(hostlist)
//sortable.Sort()
//v.HostList = []string(sortable)

//}

//func (v *View) RemovePeerFromHostlist(name string) {
//m := make(map[string]bool)
//for _, h := range v.HostList {
//if h != name {
//m[h] = true
//}
//}
//hostlist := make([]string, 0, len(m))

//for h := range m {
//hostlist = append(hostlist, h)
//}

//sortable := sort.StringSlice(hostlist)
//sortable.Sort()
//v.HostList = []string(sortable)
//}

//func (v *View) RemovePeer(name string) bool {
//dbg.Print("LOOKING FOR", name, "in HOSTLIST", v.HostList)
//v.Lock()
//// make sure we don't remove our parent
//if v.Parent == name {
//v.Unlock()
//return false
//}
//v.Unlock()

//removed := v.RemoveChild(name)

//v.Lock()
//defer v.Unlock()
//if len(v.HostList) == 0 {
//return false
//}

//v.RemovePeerFromHostlist(name)
//return removed
//}

type Views struct {
	sync.RWMutex
	Views map[int]*View
}

func NewViews() *Views {
	vs := &Views{Views: make(map[int]*View)}
	return vs
}

// Returns the parent for this view
func (v *Views) Parent(view int) string {
	v.RLock()
	defer v.RUnlock()
	return v.Views[view].Parent
}

// Returns the peer list for this view
func (v *Views) HostList(view int) PeerList {
	v.RLock()
	defer v.RUnlock()
	if v.Views[view] == nil {
		return nil
	}
	return v.Views[view].PeerList
}

func (v *Views) Children(view int) []*Node {
	v.RLock()
	defer v.RUnlock()
	if view < len(v.Views) {
		return v.Views[view].Children
	} else {
		return nil
	}
}

func (v *Views) NChildren(view int) int {
	v.RLock()
	defer v.RUnlock()
	return v.Views[view].NChildren()
}
