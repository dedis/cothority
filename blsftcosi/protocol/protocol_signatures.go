package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

type responseProtocol struct {
	structResponse StructResponse
	subProtocol    *SubBlsFtCosi
}

func (p *BlsFtCosi) checkFailureThreshold(numFailure int) bool {
	if numFailure == 0 {
		return false
	}

	return numFailure > len(p.Roster().List)-p.Threshold
}

// Collect signatures from each sub-leader, restart whereever sub-leaders fail to respond.
// The collected signatures are already aggregated for a particular group
func (p *BlsFtCosi) collectSignatures() (map[network.ServerIdentityID]*Response, error) {
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
	responseMap := make(map[network.ServerIdentityID]*Response)
	numSignature := 0
	numFailure := 0
	if len(p.subProtocols) > 0 {
		for numSignature < p.Threshold-1 && !p.checkFailureThreshold(numFailure) {
			select {
			case res := <-responsesChan:
				mask, err := cosi.NewMask(p.suite.(cosi.Suite), p.Roster().Publics(), nil)
				if err != nil {
					return nil, err
				}
				err = mask.SetMask(res.structResponse.Mask)
				if err != nil {
					return nil, err
				}

				numSignature += mask.CountEnabled()
				numFailure += len(res.subProtocol.List()) - 1 - mask.CountEnabled()
				responseMap[res.structResponse.ServerIdentity.ID] = &res.structResponse.Response
			case err := <-errChan:
				err = fmt.Errorf("error in getting responses: %s", err)
				close(closingChan)
				return nil, err
			case <-time.After(p.Timeout):
				close(closingChan)
				return nil, fmt.Errorf("not enough replies from nodes at timeout %v "+
					"for Threshold %d, got %d responses for %d requests", p.Timeout,
					p.Threshold, numSignature, len(p.Roster().List)-1)
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
		return nil, fmt.Errorf("failed to collect responses with errors %v", errs)
	}
	if p.checkFailureThreshold(numFailure) {
		return nil, fmt.Errorf("too many refusals (got %d), the threshold of %d cannot be achieved",
			numFailure, p.Threshold)
	}

	return responseMap, nil
}
