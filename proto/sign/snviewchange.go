package sign

import "log"

func (sn *Node) ChangeView(vcv *ViewChangeVote) {
	// log.Println(sn.Name(), " in CHANGE VIEW")
	// at this point actions have already been applied
	// all we need to do is switch our default view
	sn.viewmu.Lock()
	sn.ViewNo = vcv.View
	sn.viewmu.Unlock()
	if sn.RootFor(vcv.View) == sn.Name() {
		log.Println(sn.Name(), "Change view to root", "children", sn.Children(vcv.View))
		sn.viewChangeCh <- "root"
	} else {
		log.Println(sn.Name(), "Change view to regular")
		sn.viewChangeCh <- "regular"
	}

	sn.viewmu.Lock()
	sn.ChangingView = false
	sn.viewmu.Unlock()
	log.Println("VIEW CHANGED")
	// TODO: garbage collect old connections
}

