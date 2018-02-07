"use strict";

const net = require("../net");
const protobuf = require("../protobuf");
const misc = require("../misc");

const skipchainID = "Skipchain";

class SkipchainClient {

    /**
     * Returns a new skipchain client from a roster
     *
     * @param {cothority.Roster} roster the roster over which the client can talk to
     * @param {string} last know skipblock ID in hexadecimal format
     * @returns {SkipchainClient} A client that can talks to the skipchain services
     */
    constructor(group, roster,lastID) {
        this.lastRoster = roster;
        this.lastID = misc.hexToUint8Array(lastID);
        this.socket = new net.RosterSocket(this.lastRoster);
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
    latestBlock() {
        const requestName = "GetUpdateChain";
        const responseName = "GetUpdateChainReply";
        const request = { latestId: this.lastID };
        const client = this;
        const promise = new Promise(function(resolve,reject) {
            socket.send(requestName,responseName,request).then( (data) => {
                [lastBlock,err] = client.verifyUpdateChainReply(data);
                if (!err) {
                    reject(err);
                }
                resolve(lastBlock);
            }).catch((err) => { reject(err); });
        });
        return promise;
    }

    update(lastBlock) {
        this.lastRoster = identity.Roster.fromProtobuf(lastBlock.Roster);
    }
   /**
    * verifies if the list of skipblock given is correct and if it links with the last know given skipblockID.
    *
    * @param {Uint8Array} lastID last know skipblock ID
    * @param {GetUpdateChainReply} updateChainReply the response from a conode containing the blocks
    * @returns {(SkipBlock,err)} the last skipblock and an error if any
    */
    verifyUpdateChainReply(updateChainReply) {
        const blocks = updateChainReply.update;
        if (blocks.length == 0) {
            return [null,new Error("no block returned in the chain")];
        }
        // first verify the first block is the one we know
        const first = blocks[0];
        const id = new Uint8Array(first.hash);
        if (!misc.uint8ArrayCompare(id,this.lastID)) {
            return [null,new Error("the first ID is not the one we have")];
        }

        if (blocks.length == 1) {
            return [first,null];
        }
        // then check the block links consecutively
        var currBlock = first;
        for (var i = 1; i < blocks.length; i++) {
            const nextBlock = blocks[i];

            const forwardLinks = currBlock.forward;
            if (forwardLinks.length == 0)
                return [null,new Error("No forward links included in the skipblocks")];

            // only take the highest link since we move "as fast as possible" on
            // the skipchain, i.e. we skip the biggest number of blocks
            const lastLink = forwardLinks[forwardLinks.length-1];
            // XXX to change later to source_hash, dest_hash, dst_roster_id
            const message = nextBlock.hash;
            const roster = identity.Roster.fromProtobuf(currBlock.roster);
            if (!this.verifyForwardLink(roster,message,lastLink))
                return [null,new Error("Forward link incorrect!")];

            // move to the next block
            currBlock = nextBlock;
        }
    }

    /**
     * verify if the link is a valid signature over the given message for the given roster
     *
     * @param {Roster} roster THe roster who created the signature
     * @param {Uint8Array} message the message
     * @param {Object} link BlockLink object (protobuf)
     * @returns {Boolean} true if signature is valid, false otherwise
     */
    function verifyForwardLink(roster, message, link) {

    }
}


