package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/dedis/onet/log"
)

type responseProtocol struct {
	structResponse StructResponse
	subProtocol    *SubBlsFtCosi
}

// Collect signatures from each sub-leader, restart whereever sub-leaders fail to respond.
// The collected signatures are already aggregated for a particular group
func (p *BlsFtCosi) collectSignatures() ([]StructResponse, []*SubBlsFtCosi, error) {
	responsesChan := make(chan responseProtocol, 2*len(p.subProtocols))
	errChan := make(chan error, len(p.subProtocols))
	closingChan := make(chan bool)

	// receive in parallel
	var closingWg sync.WaitGroup
	closingWg.Add(len(p.subProtocols))
	for i, subProtocol := range p.subProtocols {
		go func(i int, subProtocol *SubBlsFtCosi) {
			defer closingWg.Done()
			timeout := time.After(p.Timeout)
			for {
				select {
				case <-closingChan:
					return
				case <-subProtocol.subleaderNotResponding:
					subleaderID := p.subTrees[i].Root.Children[0].RosterIndex
					log.Lvlf2("(subprotocol %v) subleader with id %d failed, restarting subprotocol", i, subleaderID)

					// generate new tree by adding the current subleader to the end of the
					// leafs and taking the first leaf for the new subleader.
					nodes := []int{p.subTrees[i].Root.RosterIndex}
					for _, child := range p.subTrees[i].Root.Children[0].Children {
						nodes = append(nodes, child.RosterIndex)
					}

					if len(nodes) < 2 || subleaderID > nodes[1] {
						errChan <- fmt.Errorf("(subprotocol %v) failed with every subleader, ignoring this subtree",
							i)
						return
					}
					nodes = append(nodes, subleaderID)

					var err error
					p.subTrees[i], err = genSubtree(p.subTrees[i].Roster, nodes)
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in tree generation: %v", i, err)
						return
					}

					// restart subprotocol
					// send stop signal to old protocol
					subProtocol.HandleStop(StructStop{subProtocol.TreeNode(), Stop{}})
					subProtocol, err = p.startSubProtocol(p.subTrees[i])
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in restarting of subprotocol: %s", i, err)
						return
					}
					p.subProtocols[i] = subProtocol
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
	sharedMask, err := NewMask(p.suite, p.Roster().Publics(), -1)
	if err != nil {
		close(closingChan)
		return nil, nil, err
	}
	responseMap := make(map[*SubBlsFtCosi]StructResponse, len(p.subProtocols))
	thresholdReached := true
	thresholdReachable := true
	if len(p.subProtocols) > 0 {
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
		sign, _ := signedByteSliceToPoint(p.suite, response.CoSiReponse)
		if !sign.Equal(p.suite.G1().Point()) {
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
