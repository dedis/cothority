package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// Collect signatures from each sub-leader, restart whereever sub-leaders fail to respond.
// The collected signatures are already aggregated for a particular group
func (p *BlsFtCosi) collectSignatures(trees []*onet.Tree,
	subProtocols []*SubBlsFtCosi, publics []kyber.Point) ([]StructResponse, []*SubBlsFtCosi, error) {

	type responseProtocol struct {
		structResponse StructResponse
		subProtocol    *SubBlsFtCosi
	}

	responsesChan := make(chan responseProtocol, 2*len(subProtocols))
	errChan := make(chan error, len(subProtocols))
	closingChan := make(chan bool)

	// receive in parallel
	var closingWg sync.WaitGroup
	closingWg.Add(len(subProtocols))
	for i, subProtocol := range subProtocols {
		go func(i int, subProtocol *SubBlsFtCosi) {
			defer closingWg.Done()
			timeout := time.After(p.Timeout)
			for {
				select {
				case <-closingChan:
					return
				case <-subProtocol.subleaderNotResponding:
					subleaderID := trees[i].Root.Children[0].RosterIndex
					log.Lvlf2("(subprotocol %v) subleader with id %d failed, restarting subprotocol", i, subleaderID)

					// generate new tree by adding the current subleader to the end of the
					// leafs and taking the first leaf for the new subleader.
					nodes := []int{trees[i].Root.RosterIndex}
					for _, child := range trees[i].Root.Children[0].Children {
						nodes = append(nodes, child.RosterIndex)
					}

					if len(nodes) < 2 || subleaderID > nodes[1] {
						errChan <- fmt.Errorf("(subprotocol %v) failed with every subleader, ignoring this subtree",
							i)
						return
					}
					nodes = append(nodes, subleaderID)

					var err error
					trees[i], err = genSubtree(trees[i].Roster, nodes)
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in tree generation: %v", i, err)
						return
					}

					// restart subprotocol
					// send stop signal to old protocol
					subProtocol.HandleStop(StructStop{subProtocol.TreeNode(), Stop{}})
					subProtocol, err = p.startSubProtocol(trees[i])
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in restarting of subprotocol: %s", i, err)
						return
					}
					subProtocols[i] = subProtocol
				case response := <-subProtocol.subResponse:
					responsesChan <- responseProtocol{response, subProtocol}
					timeout = make(chan time.Time) // deactivate timeout
				case <-timeout:
					// This should never happen, as the subProto should return before that
					// timeout, even if it didn't receive enough responses.
					errChan <- fmt.Errorf("timeout should not happen while waiting for response: %d", i)
					return
				}
			}
		}(i, subProtocol)
	}

	// handle answers from all parallel threads
	sharedMask, err := NewMask(p.pairingSuite, publics, -1)
	if err != nil {
		close(closingChan)
		return nil, nil, err
	}
	responseMap := make(map[*SubBlsFtCosi]StructResponse, len(subProtocols))
	thresholdReached := true
	thresholdReachable := true
	if len(subProtocols) > 0 {
		thresholdReached = false
		for !thresholdReached && thresholdReachable {
			select {
			case res := <-responsesChan:
				// If there is a response, add to map.
				// This assumes that the last response of a subtree is the biggest one.
				// TODO- Check if this should be changed??
				responseMap[res.subProtocol] = res.structResponse

				// check if threshold is reachable
				if sumRefusals(responseMap) > len(p.Roster().Publics())-p.Threshold {
					// we assume the root accepts the proposal
					thresholdReachable = false
				}

				newMask, err := AggregateMasks(sharedMask.Mask(), res.structResponse.Mask)
				if err != nil {
					err = fmt.Errorf("error in aggregation of response masks: %s", err)
					close(closingChan)
					return nil, nil, err
				}
				err = sharedMask.SetMask(newMask)
				if err != nil {
					err = fmt.Errorf("error in setting of shared masks: %s", err)
					close(closingChan)
					return nil, nil, err
				}
				if sharedMask.CountEnabled() >= p.Threshold-1 { // we assume the root accepts the proposal
					thresholdReached = true
				}
			case err := <-errChan:
				err = fmt.Errorf("error in getting responses: %s", err)
				close(closingChan)
				return nil, nil, err
			case <-time.After(p.Timeout):
				close(closingChan)
				return nil, nil, fmt.Errorf("not enough replies from nodes at timeout %v "+
					"for Threshold %d, got %d responses and %d refusals", p.Timeout,
					p.Threshold, sharedMask.CountEnabled(), sumRefusals(responseMap))
			}
		}
	}
	close(closingChan)
	closingWg.Wait()
	close(responsesChan)
	close(errChan)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("failed to collect responses with errors %v", errs)
	}
	if !thresholdReachable {
		return nil, nil, fmt.Errorf("too many refusals (got %d), the threshold of %d cannot be achieved",
			sumRefusals(responseMap), p.Threshold)
	}

	// extract protocols and responses from map
	runningSubProtocols := make([]*SubBlsFtCosi, 0, len(responsesChan))
	responses := make([]StructResponse, 0, len(responsesChan))
	for subProtocol, response := range responseMap {
		sign, _ := signedByteSliceToPoint(p.pairingSuite, response.CoSiReponse)
		if !sign.Equal(p.pairingSuite.G1().Point()) {
			// Only pass subProtocols that have atleast one valid response in them.
			runningSubProtocols = append(runningSubProtocols, subProtocol)
			responses = append(responses, response)
		}
	}

	return responses, runningSubProtocols, nil
}

func sumRefusals(responseMap map[*SubBlsFtCosi]StructResponse) int {
	sumRefusal := 0
	for _, response := range responseMap {
		sumRefusal += len(response.Refusals)
	}
	return sumRefusal
}
