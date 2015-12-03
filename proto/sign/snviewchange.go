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

/*
func (sn *Node) ViewChange(view int, parent string, vcm *ViewChangeMessage) error {
	sn.ChangingView = true

	log.Println(sn.Name(), "VIEW CHANGE MESSAGE: new Round == , oldlsr == , view ==", vcm.Round, sn.LastSeenRound, view)
	sn.LastSeenRound = max(vcm.Round, sn.LastSeenRound)

	iAmNextRoot := false
	if sn.RootFor(vcm.ViewNo) == sn.Name() {
		iAmNextRoot = true
	}

	sn.Views().Lock()
	_, exists := sn.Views().Views[vcm.ViewNo]
	sn.Views().Unlock()
	if !exists {
		log.Println("PEERS:", sn.Peers())
		children := sn.childrenForNewView(parent)
		log.Println("CREATING NEW VIEW with", len(sn.HostListOn(view-1)), "hosts", "on view", view)
		sn.NewView(vcm.ViewNo, parent, children, sn.HostListOn(view-1))
	}

	log.Println(sn.Name(), ":multiplexing onto children:", sn.Children(view))
	sn.multiplexOnChildren(vcm.ViewNo, &SigningMessage{View: view, Type: ViewChange, Vcm: vcm})

	log.Println(sn.Name(), "waiting on view accept messages from children:", sn.Children(view))

	votes := len(sn.Children(view))

	log.Println(sn.Name(), "received view accept messages from children:", votes)

	var err error
	if iAmNextRoot {

		if votes > len(sn.HostListOn(view))*2/3 {

			log.Println(sn.Name(), "quorum", votes, "of", len(sn.HostListOn(view)), "confirmed me as new root")
			vcfm := &ViewConfirmedMessage{ViewNo: vcm.ViewNo}
			sm := &SigningMessage{Type: ViewConfirmed, Vcfm: vcfm, From: sn.Name(), View: vcm.ViewNo}
			sn.multiplexOnChildren(vcm.ViewNo, sm)

			sn.ChangingView = false
			sn.ViewNo = vcm.ViewNo
			sn.viewChangeCh <- "root"
		} else {
			log.Errorln(sn.Name(), "(ROOT) DID NOT RECEIVE quorum", votes, "of", len(sn.HostList))
			return ErrViewRejected
		}
	} else {
		sn.RoundsAsRoot = 0

		vam := &ViewAcceptedMessage{ViewNo: vcm.ViewNo, Votes: votes}

		log.Println(sn.Name(), "putting up on view", view, "accept for view", vcm.ViewNo)
		err = sn.PutUp(context.TODO(), vcm.ViewNo, &SigningMessage{
			View: view,
			From: sn.Name(),
			Type: ViewAccepted,
			Vam:  vam})

		return err
	}

	return err
}

func (sn *Node) ViewChanged(view int, sm *SigningMessage) {
	log.Println(sn.Name(), "view CHANGED to", view)

	sn.ChangingView = false

	sn.viewChangeCh <- "regular"

	log.Println("in view change, children for view", view, sn.Children(view))
	sn.multiplexOnChildren(view, sm)
	log.Println(sn.Name(), "exited view CHANGE to", view)
}
*/
