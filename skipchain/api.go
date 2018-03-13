package skipchain

import (
	"errors"
	"fmt"
	"math/rand"

	"gopkg.in/dedis/cothority.v2"
	status "gopkg.in/dedis/cothority.v2/status/service"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/sign/schnorr"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/dedis/onet.v2/network"
)

// Client is a structure to communicate with the Skipchain
// service from the outside
type Client struct {
	*onet.Client
}

// NewClient instantiates a new client with name 'n'
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, "Skipchain")}
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
//  - parent is the responsible parent-block, can be 'nil'
//  - priv is a private key that is allowed to sign for new skipblocks
//
// This function returns the created skipblock or nil and an error.
func (c *Client) CreateGenesisSignature(ro *onet.Roster, baseH, maxH int, ver []VerifierID,
	data interface{}, parent SkipBlockID, priv kyber.Scalar) (*SkipBlock, error) {
	genesis := NewSkipBlock()
	genesis.Roster = ro
	genesis.VerifierIDs = ver
	genesis.MaximumHeight = maxH
	genesis.BaseHeight = baseH
	genesis.ParentBlockID = parent
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
	return sb.Latest, nil
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
	data interface{}, parent SkipBlockID) (*SkipBlock, error) {
	return c.CreateGenesisSignature(ro, baseH, maxH, ver, data, parent, nil)
}

// CreateRootControl is a convenience function and creates two Skipchains:
// a root SkipChain with maximumHeight of maxHRoot and a control SkipChain with
// maximumHeight of maxHControl. It connects both chains for later
// reference. The root-chain will use `VerificationRoot` and the config-chain
// will use `VerificationConfig`.
//
// A slice of verification-functions is given for the root and the control
// skipchain.
func (c *Client) CreateRootControl(elRoot, elControl *onet.Roster,
	keys []kyber.Point, baseHeight,
	maxHRoot, maxHControl int) (root, control *SkipBlock, err error) {
	log.Lvl2("Creating root roster", elRoot)
	root, err = c.CreateGenesis(elRoot, baseHeight, maxHRoot,
		VerificationRoot, nil, nil)
	if err != nil {
		return
	}
	log.Lvl2("Creating control roster", elControl)
	control, err = c.CreateGenesis(elControl, baseHeight, maxHControl,
		VerificationControl, nil, root.Hash)
	if err != nil {
		return
	}
	return root, control, err
}

// GetUpdateChain will return the chain of SkipBlocks going from the 'latest' to
// the most current SkipBlock of the chain. It takes a roster that knows the
// 'latest' skipblock and the id (=hash) of the latest skipblock.
func (c *Client) GetUpdateChain(roster *onet.Roster, latest SkipBlockID) (reply *GetUpdateChainReply, err error) {
	const retries = 3

	reply = &GetUpdateChainReply{}
	for {
		r2 := &GetUpdateChainReply{}

		// Try up to retries random servers from the given roster.
		i := 0
		perm := rand.Perm(len(roster.List))
		for ; i < retries; i++ {
			// To handle the case where len(perm) < retries.
			which := i % len(perm)
			err = c.SendProtobuf(roster.List[perm[which]], &GetUpdateChain{LatestID: latest}, r2)
			if err == nil && len(r2.Update) != 0 {
				break
			}
		}
		if i == retries {
			return nil, fmt.Errorf("too many retries; last error: %v", err)
		}

		// Does this chain start where we expect it to?
		if !r2.Update[0].Hash.Equal(latest) {
			return nil, errors.New("first returned block does not match requested hash")
		}

		// Step through the returned blocks one at a time, verifying
		// the forward links, and that they link correctly backwards.
		for j, b := range r2.Update {
			if j == 0 && len(reply.Update) > 0 {
				continue
			}

			if err := b.VerifyForwardSignatures(); err != nil {
				return nil, err
			}
			// Cannot check back links until we've confirmed the first one
			if len(reply.Update) > 0 {
				if len(b.BackLinkIDs) == 0 {
					return nil, errors.New("no backlinks?")
				}
				lastHash := reply.Update[len(reply.Update)-1].Hash
				if !b.BackLinkIDs[len(b.BackLinkIDs)-1].Equal(lastHash) {
					return nil, errors.New("highest backlink does not lead to previous received update")
				}
			}

			reply.Update = append(reply.Update, b)
		}

		last := reply.Update[len(reply.Update)-1]

		// If they updated us to the end of the chain, return.
		if last.GetForwardLen() == 0 {
			return reply, nil
		}

		// Otherwise update the roster and contact the new servers
		// to continue following the chain.
		latest = last.Hash
		roster = last.Roster
	}
}

// GetAllSkipchains returns all skipchains known to that conode. If none are
// known, an empty slice is returned.
func (c *Client) GetAllSkipchains(si *network.ServerIdentity) (reply *GetAllSkipchainsReply,
	err error) {
	reply = &GetAllSkipchainsReply{}
	err = c.SendProtobuf(si, &GetAllSkipchains{}, reply)
	return
}

// GetSingleBlock searches for a block with the given ID and returns that block,
// or an error if that block is not found.
func (c *Client) GetSingleBlock(roster *onet.Roster, id SkipBlockID) (reply *SkipBlock, err error) {
	reply = &SkipBlock{}
	err = c.SendProtobuf(roster.RandomServerIdentity(),
		&GetSingleBlock{id}, reply)
	return
}

// GetSingleBlockByIndex searches for a block with the given index following the genesis-block.
// It returns that block, or an error if that block is not found.
func (c *Client) GetSingleBlockByIndex(roster *onet.Roster, genesis SkipBlockID, index int) (reply *SkipBlock, err error) {
	reply = &SkipBlock{}
	err = c.SendProtobuf(roster.RandomServerIdentity(),
		&GetSingleBlockByIndex{genesis, index}, reply)
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
