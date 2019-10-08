package skipchain

import (
	"bytes"
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3"
	status "go.dedis.ch/cothority/v3/status/service"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*onet.Client
	// Used for SendProtobufParallel. If it is nil, default values will be used.
	options *onet.ParallelOptions
}

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, "Skipchain")}
}

// UseNode sets the options so that only the given node will be contacted
func (c *Client) UseNode(n int) {
	if c.options == nil {
		c.options = &onet.ParallelOptions{}
	}
	c.options.DontShuffle = true
	c.options.StartNode = n
	c.options.AskNodes = 1
}

// DontContact adds the given serverIdentity to the list of nodes that will
// not be contacted.
func (c *Client) DontContact(si *network.ServerIdentity) {
	if c.options == nil {
		c.options = &onet.ParallelOptions{}
	}
	c.options.IgnoreNodes = []*network.ServerIdentity{si}
}

// StoreSkipBlockSignature asks the cothority to store the new skipblock, and
// eventually attach it after the target skipblock.
//  - target is a skipblock, and the new skipblock is going to be added after
//    it, not necessarily immediately after it.  The caller should use the
//    genesis skipblock as the target. But for backward compatibility, any
//    skipblock can be used.  If ro and d are nil, a new skipchain will be
//    created with target as the genesis-block.
//  - ro is the new roster for that block. If ro is nil, the previous roster
//    will be used.
//  - d is the data for the new block. It can be nil. If it is not of type
//    []byte, it will be marshalled using `network.Marshal`.
//  - priv is the private key that will be used to sign the skipblock. If priv
//    is nil, the skipblock will not be signed.
func (c *Client) StoreSkipBlockSignature(target *SkipBlock, ro *onet.Roster, d network.Message, priv kyber.Scalar) (reply *StoreSkipBlockReply, err error) {
	log.Lvlf3("%#v", target)
	var newBlock *SkipBlock
	var targetID SkipBlockID
	if ro == nil && d == nil {
		newBlock = target
	} else {
		newBlock = target.Copy()
		if ro != nil {
			newBlock.Roster = ro
		}
		if d != nil {
			var ok bool
			newBlock.Data, ok = d.([]byte)
			if !ok {
				buf, err := network.Marshal(d)
				if err != nil {
					return nil, errors.New(
						"Couldn't marshal data: " + err.Error())

				}
				newBlock.Data = buf
			}
		}
		targetID = target.Hash
	}
	host := target.Roster.Get(0)
	reply = &StoreSkipBlockReply{}
	var sig *[]byte
	if priv != nil {
		signature, err := schnorr.Sign(cothority.Suite, priv, newBlock.CalculateHash())
		if err != nil {
			return nil, errors.New("couldn't sign block: " + err.Error())
		}
		sig = &signature
	}
	err = c.SendProtobuf(host, &StoreSkipBlock{TargetSkipChainID: targetID, NewBlock: newBlock,
		Signature: sig}, reply)
	if err != nil {
		return nil, err
	}

	if reply.Latest != nil {
		if err = reply.Latest.VerifyForwardSignatures(); err != nil {
			return nil, err
		}
	}
	if reply.Previous != nil {
		if err = reply.Previous.VerifyForwardSignatures(); err != nil {
			return nil, err
		}
	}

	return reply, nil
}

// StoreSkipBlock asks the cothority to store the new skipblock, and eventually
// attach it after the target skipblock.
//  - target is a skipblock, where new skipblock is going to be added after it,
//    but not necessarily immediately after it.  The caller should use the
//    genesis skipblock as the target. But for backward compatibility, any
//    skipblock can be used.  If ro and d are nil, a new skipchain will be
//    created with target as the genesis-block.
//  - ro is the new roster for that block. If ro is nil, the previous roster
//    will be used.
//  - d is the data for the new block. It can be nil. If it is not of type
//    []byte, it will be marshalled using `network.Marshal`.
func (c *Client) StoreSkipBlock(target *SkipBlock, ro *onet.Roster, d network.Message) (reply *StoreSkipBlockReply, err error) {
	return c.StoreSkipBlockSignature(target, ro, d, nil)
}

// CreateGenesisSignature is a convenience function to create a new SkipChain with the
// given parameters.
//  - ro is the responsible roster
//  - baseH is the base-height - the distance between two non-height-1 skipblocks
//  - maxH is the maximum height, which must be <= baseH
//  - ver is a slice of verifications to apply to that block
//  - data can be nil or any data that will be network.Marshaled to the
//    skipblock, except if the data is of type []byte, in which case it will be
//    stored as-is on the skipchain.
//  - priv is a private key that is allowed to sign for new skipblocks
//
// This function returns the created skipblock or nil and an error.
func (c *Client) CreateGenesisSignature(ro *onet.Roster, baseH, maxH int, ver []VerifierID,
	data interface{}, priv kyber.Scalar) (*SkipBlock, error) {
	genesis := NewSkipBlock()
	genesis.Roster = ro
	genesis.VerifierIDs = ver
	genesis.MaximumHeight = maxH
	genesis.BaseHeight = baseH
	if data != nil {
		var ok bool
		genesis.Data, ok = data.([]byte)
		if !ok {
			buf, err := network.Marshal(data)
			if err != nil {
				return nil, errors.New(
					"Couldn't marshal data: " + err.Error())

			}
			genesis.Data = buf
		}
	}
	sb, err := c.StoreSkipBlockSignature(genesis, nil, nil, priv)
	if err != nil {
		return nil, err
	}

	// at this point we only know that the hash is correct but we need to compare
	// what is inside the block (e.g. roster, verifiers, ...) because the distant
	// node could have changed that
	err = compareGenesisBlocks(genesis, sb.Latest)
	if err != nil {
		return nil, err
	}

	return sb.Latest, nil
}

// CompareGenesisBlocks compares the content of two blocks and returns an error
// if there is any difference
func compareGenesisBlocks(prop *SkipBlock, ret *SkipBlock) error {
	if ret == nil {
		return errors.New("got an empty reply")
	}

	ok, err := ret.Roster.Equal(prop.Roster)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("got a different roster")
	}

	if !VerifierIDs(ret.VerifierIDs).Equal(prop.VerifierIDs) {
		return errors.New("got a different list of verifiers")
	}

	if !bytes.Equal(ret.Data, prop.Data) {
		return errors.New("data field does not match")
	}

	if ret.MaximumHeight != prop.MaximumHeight {
		return errors.New("got a different maximum height")
	}

	if ret.BaseHeight != prop.BaseHeight {
		return errors.New("got a different base height")
	}

	return nil
}

// CreateGenesis is a convenience function to create a new SkipChain with the
// given parameters.
//  - ro is the responsible roster
//  - baseH is the base-height - the distance between two non-height-1 skipblocks
//  - maxH is the maximum height, which must be <= baseH
//  - ver is a slice of verifications to apply to that block
//  - data can be nil or any data that will be network.Marshaled to the skipblock,
//    except if the data is of type []byte, in which case it will be stored
//    as-is on the skipchain.
//  - parent is the responsible parent-block, can be 'nil'
//
// This function returns the created skipblock or nil and an error.
func (c *Client) CreateGenesis(ro *onet.Roster, baseH, maxH int, ver []VerifierID,
	data interface{}) (*SkipBlock, error) {
	return c.CreateGenesisSignature(ro, baseH, maxH, ver, data, nil)
}

// OptimizeProof asks for the proof of the block ID to the roster and creates
// missing forward-links if any.
func (c *Client) OptimizeProof(ro *onet.Roster, id SkipBlockID) (*OptimizeProofReply, error) {
	reply := &OptimizeProofReply{}

	si := ro.RandomServerIdentity()
	err := c.SendProtobuf(si, &OptimizeProofRequest{
		Roster: ro,
		ID:     id,
	}, reply)

	return reply, err
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain. It takes a roster that knows the
// 'latest' skipblock and the id (=hash) of the latest skipblock.
// The returned list of blocks is linked using the highest level links available
// to shorten the returned chain.
func (c *Client) GetUpdateChain(roster *onet.Roster, latest SkipBlockID) (reply *GetUpdateChainReply, err error) {
	update, err := c.GetUpdateChainLevel(roster, latest, -1, -1)
	if err != nil {
		return nil, err
	}
	return &GetUpdateChainReply{update}, nil
}

// GetUpdateChainLevel will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain. It takes a roster that knows the
// 'latest' skipblock and the id (=hash) of the latest known skipblock.
//   - roster: which nodes to contact to get updates
//   - latest: the latest known block
//   - maxLevel: what maximum height to use. -1 means the highest height available.
//     0 means only direct forward links, n means level-n forwardlinks.
//   - maxBlocks: how many blocks to return at maximum.
func (c *Client) GetUpdateChainLevel(roster *onet.Roster, latest SkipBlockID,
	maxLevel int, maxBlocks int) (update []*SkipBlock, err error) {
	for {
		r2 := &GetUpdateChainReply{}

		_, err = c.SendProtobufParallel(roster.List, &GetUpdateChain{
			LatestID:  latest,
			MaxHeight: maxLevel,
			MaxBlocks: maxBlocks - len(update),
		}, r2, c.options)
		if err != nil {
			return nil, fmt.Errorf("couldn't get update chain; last error: %v", err)
		}

		// Does this chain start where we expect it to?
		if !r2.Update[0].Hash.Equal(latest) {
			return nil, errors.New("first returned block does not match requested hash")
		}

		// Step through the returned blocks one at a time, verifying
		// the forward links, and that they link correctly backwards.
		for j, b := range r2.Update {
			if j == 0 && len(update) > 0 {
				if update[len(update)-1].Hash.Equal(b.Hash) {
					continue
				}
			}

			// Check the integrity of the block
			if err := b.VerifyForwardSignatures(); err != nil {
				return nil, err
			}
			// Cannot check back links until we've confirmed the first one
			if len(update) > 0 {
				if len(b.BackLinkIDs) == 0 {
					return nil, errors.New("no backlinks?")
				}
				// To verify a correct backlink, there are a couple of corner cases.
				// Let's suppose the last 4 blocks of a skipchain have the following
				// index:heights, and n is the last block of the skipchain:
				// n-3:2 n-2:1 n-1:3 n:1
				// GetUpdateChain will return [n-3, n-1, n] and won't send n-2. So going
				// from n-3 to n-1, we need to check the backlink at height 2. From
				// n-1 to n, we need to check the backlink at height 1.
				// Summary: we need to check the backlink of the minimal height between the
				// two blocks.
				prevBlock := update[len(update)-1]
				link := prevBlock.Height
				if link > b.Height {
					link = b.Height
				}
				if !b.BackLinkIDs[link-1].Equal(prevBlock.Hash) {
					return nil, errors.New("corresponding backlink doesn't point to previous block")
				}
				if !prevBlock.ForwardLink[link-1].To.Equal(b.Hash) {
					return nil, errors.New("corresponding forwardlink doesn't point to next block")
				}
			}
			update = append(update, b)
			if len(update) == maxBlocks {
				return update, nil
			}
		}

		last := update[len(update)-1]

		// If they updated us to the end of the chain, return.
		if last.GetForwardLen() == 0 {
			return update, nil
		}

		// Otherwise update the roster and contact the new servers
		// to continue following the chain.
		flHeight := len(last.ForwardLink)
		if maxLevel > 0 && flHeight > maxLevel {
			flHeight = maxLevel
		}
		highestFL := last.ForwardLink[flHeight-1]
		latest = highestFL.To
		roster = highestFL.NewRoster
		if roster == nil {
			roster = last.Roster
		}
	}
}

// GetAllSkipchains is deprecated and should no longer be used. See GetAllSkipChainIDs.
func (c *Client) GetAllSkipchains(si *network.ServerIdentity) (reply *GetAllSkipchainsReply,
	err error) {
	reply = &GetAllSkipchainsReply{}
	err = c.SendProtobuf(si, &GetAllSkipchains{}, reply)
	return
}

// GetAllSkipChainIDs returns the SkipBlockIDs of all of the genesis
// blocks of all skipchains known to that conode.
func (c *Client) GetAllSkipChainIDs(si *network.ServerIdentity) (reply *GetAllSkipChainIDsReply,
	err error) {
	reply = &GetAllSkipChainIDsReply{}
	err = c.SendProtobuf(si, &GetAllSkipChainIDs{}, reply)
	return
}

// GetSingleBlock searches for a block with the given ID and returns that block,
// or an error if that block is not found.
func (c *Client) GetSingleBlock(roster *onet.Roster, id SkipBlockID) (*SkipBlock, error) {
	var reply = &SkipBlock{}
	_, err := c.SendProtobufParallel(roster.List, &GetSingleBlock{id}, reply, c.options)
	if err != nil {
		return nil, errors.New("all nodes failed to return block: " + err.Error())
	}
	if err := reply.VerifyForwardSignatures(); err != nil {
		return nil, err
	}

	if !reply.Hash.Equal(id) {
		return nil, errors.New("Got the wrong block in return")
	}

	return reply, nil
}

// GetSingleBlockByIndex searches for a block with the given index following the genesis-block.
// It returns that block, or an error if that block is not found.
func (c *Client) GetSingleBlockByIndex(roster *onet.Roster, genesis SkipBlockID, index int) (reply *GetSingleBlockByIndexReply, err error) {
	reply = &GetSingleBlockByIndexReply{}

	_, err = c.SendProtobufParallel(roster.List, &GetSingleBlockByIndex{genesis, index}, reply,
		c.options)
	if err != nil {
		return
	}

	if reply.SkipBlock == nil {
		err = errors.New("got an empty reply")
		return
	}

	if err = reply.SkipBlock.VerifyForwardSignatures(); err != nil {
		return
	}

	if reply.SkipBlock.Index != index {
		err = errors.New("got the wrong block in reply")
		return
	}

	if !reply.SkipBlock.SkipChainID().Equal(genesis) {
		err = errors.New("got a block of a different chain")
		return
	}

	return
}

// CreateLinkPrivate asks the conode to create a link by sending a public
// key of the client, signed by the private key of the conode. The reasoning is
// that an administrator should well be able to copy the private.toml-file from
// the server and use that to authenticate and link to the conode.
func (c *Client) CreateLinkPrivate(si *network.ServerIdentity, conodePriv kyber.Scalar,
	pub kyber.Point) error {
	reply := &EmptyReply{}
	msg, err := pub.MarshalBinary()
	if err != nil {
		return errors.New("couldn't marshal point: " + err.Error())
	}
	sig, err := schnorr.Sign(cothority.Suite, conodePriv, msg)
	if err != nil {
		return errors.New("couldn't sign public key: " + err.Error())
	}
	return c.SendProtobuf(si, &CreateLinkPrivate{Public: pub, Signature: sig}, reply)
}

// Unlink removes a link on the remote service for our client. This might be
// because we want to change the key. It's not possible to remove a lost key,
// only if you have the private key can you request to remove the public
// counterpart on the server.
func (c *Client) Unlink(si *network.ServerIdentity, priv kyber.Scalar) error {
	public := cothority.Suite.Point().Mul(priv, nil)
	msg, err := public.MarshalBinary()
	if err != nil {
		return err
	}
	msg = append([]byte("unlink:"), msg...)
	sig, err := schnorr.Sign(cothority.Suite, priv, msg)
	if err != nil {
		return err
	}
	return c.SendProtobuf(si, &Unlink{
		Public:    public,
		Signature: sig,
	}, &EmptyReply{})
}

// Listlink returns all public keys that are allowed to contact
// this conode securely. It can return an empty list which means
// that this conode is not secured.
func (c *Client) Listlink(si *network.ServerIdentity) ([]kyber.Point, error) {
	reply := &ListlinkReply{}
	err := c.SendProtobuf(si, &Listlink{}, reply)
	if err != nil {
		return nil, err
	}
	return reply.Publics, nil
}

// AddFollow gives a skipchain-id to the conode that should be used to allow/disallow
// new blocks. Only if SettingAuthentication(true) has been called is this active.
// The Follow is one of: 0 - only allow this skipchain to add new blocks.
// 1 - search if it can find that skipchain-id and then add the whole roster to
// the list of allowed nodes to request a new skipblock. 2 - lookup the skipchain-id
// given the ip and port of the conode where it is available.
func (c *Client) AddFollow(si *network.ServerIdentity, clientPriv kyber.Scalar,
	scid SkipBlockID, Follow FollowType, NewChain PolicyNewChain, conode string) error {
	req := &AddFollow{
		SkipchainID: scid,
		Follow:      Follow,
		NewChain:    NewChain,
	}
	msg := []byte{byte(Follow)}
	msg = append(scid, msg...)

	if conode != "" {
		log.Lvl2("Getting public key of conode:", conode, si)
		lookup := network.NewServerIdentity(cothority.Suite.Point().Null(), network.NewAddress(network.PlainTCP, conode))
		resp, err := status.NewClient().Request(lookup)
		if err != nil {
			return err
		}
		req.Conode = resp.ServerIdentity
		msg = append(msg, req.Conode.ID[:]...)
	}
	sig, err := schnorr.Sign(cothority.Suite, clientPriv, msg)
	if err != nil {
		return errors.New("couldn't sign message:" + err.Error())
	}
	req.Signature = sig
	return c.SendProtobuf(si, req, nil)
}

// DelFollow asks the conode to remove a skipchain-id from the list of skipchains that are
// used to to allow/disallow new blocks. Only if SettingAuthentication(true) has
// been called is this active.
func (c *Client) DelFollow(si *network.ServerIdentity, clientPriv kyber.Scalar, scid SkipBlockID) error {
	msg := append([]byte("delfollow:"), scid...)
	sig, err := schnorr.Sign(cothority.Suite, clientPriv, msg)
	if err != nil {
		return err
	}
	return c.SendProtobuf(si, &DelFollow{SkipchainID: scid, Signature: sig}, nil)
}

// ListFollow returns the list of latest skipblock of all skipchains that are followed
// for authentication purposes.
func (c *Client) ListFollow(si *network.ServerIdentity, clientPriv kyber.Scalar) (*ListFollowReply, error) {
	msg, err := si.Public.MarshalBinary()
	if err != nil {
		return nil, err
	}
	msg = append([]byte("listfollow:"), msg...)
	sig, err := schnorr.Sign(cothority.Suite, clientPriv, msg)
	if err != nil {
		return nil, err
	}
	reply := &ListFollowReply{}
	err = c.SendProtobuf(si, &ListFollow{Signature: sig}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
