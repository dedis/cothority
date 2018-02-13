//"use strict";

const net = require("../net");
const protobuf = require("../protobuf");
const misc = require("../misc");
const identity = require("../identity.js");

const kyber = require("@dedis/kyber-js");

const co = require("co");

const skipchainID = "Skipchain";

class Client {
  /**
   * Returns a new skipchain client from a roster
   *
   * @param {cothority.Roster} roster the roster over which the client can talk to
   * @param {string} last know skipblock ID in hexadecimal format
   * @returns {SkipchainClient} A client that can talks to the skipchain services
   */
  constructor(group, roster, lastID) {
    this.lastRoster = roster;
    this.lastID = misc.hexToUint8Array(lastID);
    this.group = group;
  }

  /**
   * Returns the latest known skipblockID
   *
   * @returns {string} hexadecimal encoded skipblockID
   */
  get latestID() {
    return misc.uint8ArrayToHex(this.lastID);
  }

  /**
   * updateChain asks for the latest block of the skipchain with all intermediate blocks.
   * It automatically verifies the transition from the last known skipblock ID to the
   * latest one returned. It also automatically save the latest good known
   * roster from the latest block.
   * @return {Promise} A promise which resolves with the latest skipblock if
   * all checks pass.
   */
  getLatestBlock() {
    var fn = co.wrap(function*(client) {
      const requestStr = "GetUpdateChain";
      const responseStr = "GetUpdateChainReply";
      const request = { latestId: client.lastID };
      // XXX  somewhat hackyish but sets a realistic upper bound
      const initLength = client.lastRoster.length;
      var nbErr = 0;
      while (nbErr < initLength) {
        // fetches the data with the current roster
        client.socket = new net.RosterSocket(client.lastRoster, skipchainID);
        var data = null;
        try {
          data = yield client.socket.send(requestStr, responseStr, request);
        } catch (err) {
          return Promise.reject(err);
        }
        // verifies it's all correct
        var lastBlock, err;
        [lastBlock, err] = client.verifyUpdateChainReply(data);
        if (!err) {
          // tries again with random conodes
          nbErr++;
        }
        // update the roster
        client.lastRoster = identity.Roster.fromProtobuf(lastBlock.roster);
        client.lastID = lastBlock.hash;
        // if there is nothing new stop !
        if (!lastBlock.forward || lastBlock.forward.length == 0) {
          // no forward block means it's the latest block
          return Promise.resolve(lastBlock);
        }
      }
      return Promise.reject(nbErr + " occured retrieving the latest block...");
    });
    return fn(this);
  }

  /**
   * verifies if the list of skipblock given is correct and if it links with the last know given skipblockID.
   *
   * @param {Uint8Array} lastID last know skipblock ID
   * @param {GetUpdateChainReply} updateChainReply the response from a conode containing the blocks
   * @returns {(SkipBlock,err)} the most recent valid block in the chain, or an error
   */
  verifyUpdateChainReply(updateChainReply) {
    console.log("Verifying update...");
    const blocks = updateChainReply.update;
    if (blocks.length == 0) {
      return [null, new Error("no block returned in the chain")];
    }
    // first verify the first block is the one we know
    const first = blocks[0];
    const id = new Uint8Array(first.hash);
    if (!misc.uint8ArrayCompare(id, this.lastID)) {
      return [null, new Error("the first ID is not the one we have")];
    }

    if (blocks.length == 1) {
      return [first, null];
    }
    // then check the block links consecutively
    var currBlock = first;
    for (var i = 1; i < blocks.length; i++) {
      const nextBlock = blocks[i];

      const forwardLinks = currBlock.forward;
      if (forwardLinks.length == 0)
        return [null, new Error("No forward links included in the skipblocks")];

      // only take the highest link since we move "as fast as possible" on
      // the skipchain, i.e. we skip the biggest number of blocks
      const lastLink = forwardLinks[forwardLinks.length - 1];
      // XXX to change later to source_hash, dest_hash, dst_roster_id
      const message = nextBlock.hash;
      const roster = identity.Roster.fromProtobuf(currBlock.roster);
      //var err = this.verifyForwardLink(roster, message, lastLink);
      //if (err) return [null, err];

      // move to the next block
      currBlock = nextBlock;
    }
    return [currBlock, null];
  }

  /**
   * verify if the link is a valid signature over the given message for the given roster
   *
   * @param {Roster} roster THe roster who created the signature
   * @param {Uint8Array} message the message
   * @param {Object} link BlockLink object (protobuf)
   * @returns {Boolean} true if signature is valid, false otherwise
   */
  verifyForwardLink(roster, message, link) {
    // verify the signature length and get the bitmask
    const sigLen = link.signature.length;
    const pointLen = group.pointLen();
    const scalarlLen = group.scalarLen();
    if (link && link.signature.length < pointLen + scalarLen)
      return new Error("signature length invalid");

    // compute the bitmask and the reduced public key
    const bitmask = link.signature.slice(
      pointLen + scalarLen,
      link.signature.length
    );
    const bitmaskLenth = getBitmaskLength(bitmask);
    if (bitmaskLength > roster.length)
      return new Error("bitmask length invalid");

    const threshold = (roster.length - 1) / 3;
    if (bitmaskLength > threshold)
      return new Error("nb of signers absent above threshold");

    // get the roster aggregate key and subtract any exception listed.
    const aggregate = roster.aggregateKey();

    // all indices of the absent nodes from the roster
    const absenteesIdx = getSetBits(bitmask);
    // compute reduced public key
    absenteesIdx.forEach(idx => {
      aggregate.sub(aggregate, roster.get(idx));
    });

    // commitment to subtract from the signature
    const absentCommitment = this.group.point().null();
    if (link.exceptions) {
      const excLength = link.exceptions.length;
      for (var i = 0; i < excLength; i++) {
        var exception = link.exceptions[i];
        // subtract the absent public key from the roster aggregate key
        aggregate.sub(aggregate, roster.get(exception.index));
        // aggregate all the absent commitment
        var individual = group.point();
        individual.unmarshalBinary(exception.commitment);
        absentCommitment.add(absentCommitment, individual);
      }
    }

    // XXX suppose c = H(R || Pub || m) , with R being the FULL commitment
    // that is being generated at challenge time and the signature is
    // R' || s with R' being the reduced commitment
    // R' = R - SUM(exception-commitment)
    const R = group.point();
    R.unmarshalBinary(link.signature.slice(0, pointLen));
    const reducedR = R.clone();
    reducedR.sub(reducedR, commitment);
    const s = group.scalar();
    s.unmarshalBinary(link.signature.slice(pointLen, pointLen + scalarLen));

    // recompute challenge = H(R || P || M)
    // with P being the roster aggregate public key minus the public keys
    // indicated by the bitmask
    const buffPub = publicKey.marshalBinary();
    const challenge = schnorr.hashSchnorr(
      suite,
      R.marshalBinary(),
      aggregate.marshalBinary(),
      message
    );
    // compute sG
    const left = suite.point().mul(s, null);
    // compute R + challenge * Public
    const right = suite.point().mul(challenge, publicKey);
    right.add(right, reducedR);
    if (!right.equal(left)) {
      return new Error("invalid signature");
    }
    return null;
  }
}

module.exports.Client = Client;
