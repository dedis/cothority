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
    constructor(roster,lastID) {
        this.lastRoster = roster;
        this.lastID = misc.hexToUint8Array(lastID);
        this.socket = new net.RosterSocket(this.lastRoster);
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
                [lastBlock,err] = verifyUpdateChainReply(client.latestID,data);
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
}

/**
 * verifies if the list of skipblock given is correct and if it links with the last know given skipblockID.
 *
 * @param {Uint8Array} lastID last know skipblock ID
 * @param {GetUpdateChainReply} updateChainReply the response from a conode containing the blocks
 * @returns {(SkipBlock,err)} the last skipblock and an error if any
 */
function verifyUpdateChainReply(lastID, updateChainReply) {
    const blocks = updateChainReply.update;
    // first verify the first block is the one we know
    blocks[0]
}
