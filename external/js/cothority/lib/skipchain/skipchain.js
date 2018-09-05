"use strict";

const net = require("../net");
const protobuf = require("../protobuf");
const misc = require("../misc");
const identity = require("../identity.js");
const crypto = require("crypto");

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
   * Returns the skipblock from the skipchain, given its id.
   *
   * @param {Uint8Array} skipblockID - the id of the block
   * @return {Promise<SkipBlock>} - a promise which resolves with the specified
   * block if correct
   */
  getSkipblock(skipblockID) {
    const request = {
      id: skipblockID
    };
    let socket = new net.RosterSocket(this.lastRoster, "Skipchain");

    return socket.send("GetSingleBlock", "SkipBlock", request);
  }

  /**
   * updateChain asks for the latest block of the skipchain with all intermediate blocks.
   * It automatically verifies the transition from the last known skipblock ID to the
   * latest one returned. It also automatically remembers the latest good known
   * roster from the latest block.
   * @return {Promise} A promise which resolves with the latest skipblock if
   * all checks pass.
   */
  getLatestBlock() {
    var fn = co.wrap(function*(client) {
      const requestStr = "GetUpdateChain";
      const responseStr = "GetUpdateChainReply";
      const request = {
        latestID: client.lastID
      };
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
        var lastBlock;
        try {
          lastBlock = client.verifyUpdateChainReply(data);
        } catch (err) {
          console.log("error in the process : " + err);
          // tries again with random conodes
          nbErr++;
          continue;
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
   * @returns {SkipBlock} the most recent valid block in the chain
   * @throws {Error} throw an error if the chain is invalid
   */
  verifyUpdateChainReply(updateChainReply) {
    const blocks = updateChainReply.update;
    if (blocks.length == 0) {
      throw new Error("no block returned in the chain");
    }

    // first verify the first block is the one we know
    const first = blocks[0];
    const id = new Uint8Array(first.hash);
    if (!misc.uint8ArrayCompare(id, this.lastID)) {
      throw new Error("the first ID is not the one we have");
    }

    if (blocks.length == 1) {
      return first;
    }
    // then check the block links consecutively
    var currBlock = first;
    for (var i = 1; i < blocks.length; i++) {
      console.log(i + "/" + blocks.length);
      const nextBlock = blocks[i];

      const forwardLinks = currBlock.forward;
      if (forwardLinks.length == 0) {
        //throw new Error("No forward links included in the skipblocks");
        return currBlock;
      }

      // only take the highest link since we move "as fast as possible" on
      // the skipchain, i.e. we skip the biggest number of blocks
      const lastLink = forwardLinks[forwardLinks.length - 1];
      const roster = identity.Roster.fromProtobuf(currBlock.roster);
      var err = this.verifyForwardLink(roster, lastLink);
      //if (err) console.log("error verifying: " + err);
      if (err) {
        throw err;
      }

      // move to the next block
      currBlock = nextBlock;
    }
    return currBlock;
  }

  /**
   * verify if the link is a valid signature over the given message for the given roster
   *
   * @param {Roster} the roster who created the signature
   * @param {Uint8Array} the message
   * @param {Object} BlockLink object (protobuf)
   * @returns {error} Error in case a link is wrong
   */
  verifyForwardLink(roster, flink) {
    // check that the message is correct
    const message = flink.signature.msg;
    const h = crypto.createHash("sha256");
    h.update(flink.from);
    h.update(flink.to);
    if (flink.roster !== undefined) {
      return new Error(
        "forwardlink verification with a roster change is not implemented yet"
      );
    }
    let b = h.digest();
    let hash = new Uint8Array(
      b.buffer,
      b.byteOffset,
      b.byteLength / Uint8Array.BYTES_PER_ELEMENT
    );
    if (!misc.uint8ArrayCompare(hash, message, false)) {
      return new Error("recreated message does not match");
    }

    // verify the signature length and get the bitmask
    var bftSig = flink.signature;
    const sigLen = bftSig.sig.length;
    const pointLen = this.group.pointLen();
    const scalarLen = this.group.scalarLen();
    if (sigLen < pointLen + scalarLen)
      return new Error("signature length invalid");

    // compute the bitmask and the reduced public key
    const bitmask = bftSig.sig.slice(pointLen + scalarLen, bftSig.sig.length);
    const bitmaskLength = misc.getBitmaskLength(bitmask);
    const expectedBitmaskLength = roster.length + 8 - roster.length % 8;
    if (bitmaskLength > expectedBitmaskLength)
      return new Error("bitmask length invalid");

    const threshold = (roster.length - 1) / 3;
    // all indices of the absent nodes from the roster
    const absenteesIdx = misc.getClearBits(bitmask, roster.length);
    if (absenteesIdx.length > threshold) {
      return new Error("nb of signers absent above threshold");
    }

    // get the roster aggregate key and subtract any exception listed.
    const aggregate = roster.aggregateKey();
    //compute reduced public key
    absenteesIdx.forEach(idx => {
      aggregate.sub(aggregate, roster.get(idx).pub);
    });

    // XXX suppose c = H(R || Pub || m) , with R being the FULL commitment
    // that is being generated at challenge time and the signature is
    // R' || s with R' being the reduced commitment
    // R' = R - SUM(exception-commitment)
    const R = this.group.point();
    R.unmarshalBinary(bftSig.sig.slice(0, pointLen));
    const s = this.group.scalar();
    s.unmarshalBinary(bftSig.sig.slice(pointLen, pointLen + scalarLen));
    // recompute challenge = H(R || P || M)
    // with P being the roster aggregate public key minus the public keys
    // indicated by the bitmask
    const challenge = hashSchnorr(
      this.group,
      R.marshalBinary(),
      aggregate.marshalBinary(),
      message
    );
    // compute -(c * Aggregate)
    const mca = this.group.point().neg(aggregate);
    mca.mul(challenge, mca);
    // compute sG
    const left = this.group.point().mul(s, null);
    left.add(left, mca);
    // compute R + challenge * Public
    //const right = this.group.point().mul(challenge, aggregate);
    //right.add(right, R);
    const right = R;
    if (!right.equal(left)) {
      return new Error("invalid signature");
    }
    return null;
  }
}

/**
 *
 * hashSchnorr returns a scalar out of hashing the given inputs.
 * @param {...Uint8Array} inputs
 * @return {Scalar}
 *
 **/
function hashSchnorr(suite, ...inputs) {
  const h = crypto.createHash("sha256");
  for (let i of inputs) {
    h.update(i);
  }
  const scalar = suite.scalar();
  scalar.setBytes(Uint8Array.from(h.digest()));
  return scalar;
}

module.exports.Client = Client;
