package sign

/*
NOT WORKING - this can be implemented to have a RoundVote which
will ask for a view-change.
*/

/*
func (sn *Node) StartVotingRound(v *Vote) error {
	dbg.Lvl2(sn.Name(), "start voting round")
	sn.nRounds = sn.LastSeenRound

	// during view changes, only accept view change related votes
	if sn.ChangingView && v.Vcv == nil {
		dbg.Lvl2(sn.Name(), "start signing round: changingViewError")
		return ChangingViewError
	}

	sn.nRounds++
	v.Round = sn.nRounds
	v.Index = int(atomic.LoadInt64(&sn.LastSeenVote)) + 1
	v.Count = &Count{}
	v.Confirmed = false
	// only default fill-in view numbers when not prefilled
	if v.View == 0 {
		v.View = sn.ViewNo
	}
	if v.Av != nil && v.Av.View == 0 {
		v.Av.View = sn.ViewNo + 1
	}
	if v.Rv != nil && v.Rv.View == 0 {
		v.Rv.View = sn.ViewNo + 1
	}
	if v.Vcv != nil && v.Vcv.View == 0 {
		v.Vcv.View = sn.ViewNo + 1
	}
	return sn.StartAnnouncement(
		&AnnouncementMessage{Message: []byte("vote round"), RoundNbr: sn.nRounds, Vote: v})
}
*/
