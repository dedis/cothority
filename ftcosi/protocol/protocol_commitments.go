package protocol

import (
	"fmt"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"sync"
	"time"
)

// get all commitments, restart subprotocols where subleaders do not respond
func (p *FtCosi) collectCommitments(trees []*onet.Tree,
	subProtocols []*SubFtCosi) ([]StructCommitment, []*SubFtCosi, error) {

	type commitmentProtocol struct {
		structCommitment StructCommitment
		subProtocol      *SubFtCosi
	}

	commitmentsChan := make(chan commitmentProtocol, 2*len(subProtocols))
	errChan := make(chan error, len(subProtocols))
	closingChan := make(chan bool)

	// receive in parallel
	var closingWg sync.WaitGroup
	for i, subProtocol := range subProtocols {
		closingWg.Add(1)
		go func(i int, subProtocol *SubFtCosi) {
			defer closingWg.Done()
			timeout := time.After(p.Timeout / 2)
			for {
				select {
				case <-closingChan:
					return
				case <-subProtocol.subleaderNotResponding:
					subleaderID := trees[i].Root.Children[0].RosterIndex
					log.Lvlf2("(subprotocol %v) subleader with id %d failed, restarting subprotocol", i, subleaderID)

					// send stop signal
					subProtocol.HandleStop(StructStop{subProtocol.TreeNode(), Stop{}})

					// generate new tree
					newSubleaderID := subleaderID + 1
					if newSubleaderID >= len(trees[i].Roster.List) {
						log.Lvl2("(subprotocol %v) failed with every subleader, ignoring this subtree")
						return
					}
					var err error
					trees[i], err = genSubtree(trees[i].Roster, newSubleaderID)
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in tree generation: %v", i, err)
						return
					}

					// restart subprotocol
					subProtocol, err = p.startSubProtocol(trees[i])
					if err != nil {
						err = fmt.Errorf("(subprotocol %v) error in restarting of subprotocol: %s", i, err)
						errChan <- err
						return
					}
					subProtocols[i] = subProtocol
				case com := <-subProtocol.subCommitment:
					commitmentsChan <- commitmentProtocol{com, subProtocol}
					timeout = make(chan time.Time) //deactivate timeout
				case <-timeout:
					errChan <- fmt.Errorf("(subprotocol %v) didn't get commitment after timeout %v", i, p.Timeout)
					return
				}
			}
		}(i, subProtocol)
	}

	//handle answers from all parallel threads
	sharedMask, err := cosi.NewMask(p.suite, p.publics, nil)
	if err != nil {
		return nil, nil, err
	}
	commitmentsMap := make(map[*SubFtCosi]StructCommitment, len(subProtocols))
	thresholdReached := true
	thresholdReachable := true
	if len(subProtocols) > 0 {
		thresholdReached = false
		for !thresholdReached && thresholdReachable {
			select {
			case com := <-commitmentsChan:

				//if there is a commitment, add to map
				if !com.structCommitment.CoSiCommitment.Equal(p.suite.Point().Null()) {
					//assumes that the last commit of a subtree is the biggest one
					commitmentsMap[com.subProtocol] = com.structCommitment
				}

				//check if threshold is reachable
				if sumRefusals(commitmentsMap) > len(p.publics)-p.Threshold { // we assume the root accepts the proposal
					thresholdReachable = false
				}

				newMask, err := cosi.AggregateMasks(sharedMask.Mask(), com.structCommitment.Mask)
				if err != nil {
					err = fmt.Errorf("error in aggregation of commitment masks: %s", err)
					return nil, nil, err
				}
				err = sharedMask.SetMask(newMask)
				if err != nil {
					err = fmt.Errorf("error in setting of shared masks: %s", err)
					return nil, nil, err
				}
				if sharedMask.CountEnabled() >= p.Threshold-1 { // we assume the root accepts the proposal
					thresholdReached = true
				}
			case err := <-errChan:
				err = fmt.Errorf("error in getting commitments: %s", err)
				return nil, nil, err
			case <-time.After(p.Timeout):
				return nil, nil, fmt.Errorf("not enough replies from nodes at timeout "+
					"for Threshold %d, got %d commitments and %d refusals",
					p.Threshold, sharedMask.CountEnabled(), sumRefusals(commitmentsMap))
			}
		}
	}

	close(closingChan)
	closingWg.Wait()
	close(commitmentsChan)
	close(errChan)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("failed to collect commitments with errors %v", errs)
	}
	if !thresholdReachable {
		return nil, nil, fmt.Errorf("too many refusals (got %d), the threshold %d is no more reachable",
			sumRefusals(commitmentsMap), p.Threshold)
	}

	// extract protocols and commitments from map
	runningSubProtocols := make([]*SubFtCosi, 0, len(commitmentsChan))
	commitments := make([]StructCommitment, 0, len(commitmentsChan))
	for subProtocol, commitment := range commitmentsMap {
		runningSubProtocols = append(runningSubProtocols, subProtocol)
		commitments = append(commitments, commitment)
	}

	return commitments, runningSubProtocols, nil
}

func sumRefusals(commitmentsMap map[*SubFtCosi]StructCommitment) int {
	sumRefusal := 0
	for _, commitment := range commitmentsMap {
		sumRefusal += commitment.NRefusal
	}
	return sumRefusal
}
