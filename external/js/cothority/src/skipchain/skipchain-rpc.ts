import { IConnection, LeaderConnection, RosterWSConnection, WebSocketConnection } from "../network/connection";
import { Roster } from "../network/proto";
import {
    GetAllSkipChainIDs,
    GetAllSkipChainIDsReply,
    GetSingleBlock,
    GetSingleBlockByIndex,
    GetSingleBlockByIndexReply,
    GetUpdateChain,
    GetUpdateChainReply,
    StoreSkipBlock,
    StoreSkipBlockReply,
} from "./proto";
import { SkipBlock } from "./skipblock";

/**
 * SkipchainRPC provides basic tools to interact with a skipchain
 * with a given roster
 *
 * TODO:
 * - make SkipchainRPC with only static methods
 * - add Skipchain class that represents one single skipchain
 */
export default class SkipchainRPC {
    static serviceName = "Skipchain";

    /**
     * Create a leader connection. As the roster can change over the course of a
     * skipchain, this the leader is not always the same.
     *
     * @param roster list of nodes, the first one is taken as the leader
     */
    private static getLeader(roster: Roster): WebSocketConnection {
        return new LeaderConnection(roster, SkipchainRPC.serviceName);
    }

    private roster: Roster;
    private conn: IConnection;

    constructor(nodes: Roster | IConnection) {
        if (nodes instanceof Roster) {
            this.roster = nodes;
            this.conn = new RosterWSConnection(nodes, SkipchainRPC.serviceName);
        } else {
            this.conn = nodes.copy(SkipchainRPC.serviceName);
        }
    }

    /**
     * Create a skipchain with a base and a max height
     *
     * @param baseHeight    base height of the skipchain
     * @param maxHeight     maximum height of the skipchain
     * @returns a promise that resolves with the genesis block
     */
    createSkipchain(baseHeight: number = 4, maxHeight: number = 32): Promise<StoreSkipBlockReply> {
        if (this.roster === undefined) {
            throw new Error("Missing roster - initialize class with Roster");
        }
        const newBlock = new SkipBlock({roster: this.roster, maxHeight, baseHeight});
        const req = new StoreSkipBlock({newBlock});

        return SkipchainRPC.getLeader(this.roster).send(req, StoreSkipBlockReply);
    }

    /**
     * Add a new block to a given skipchain
     * @param gid the genesis ID of the skipchain
     * @param msg the data to include in the block
     * @throws an error if the request is not successful
     */
    addBlock(gid: Buffer, msg: Buffer): Promise<StoreSkipBlockReply> {
        if (this.roster === undefined) {
                throw new Error("Missing roster - initialize class with Roster");
            }
        const newBlock = new SkipBlock({roster: this.roster, data: msg});
        const req = new StoreSkipBlock({
            newBlock,
            targetSkipChainID: gid,
        });

        return SkipchainRPC.getLeader(this.roster).send(req, StoreSkipBlockReply);
    }

    /**
     * Get the block with the given ID
     *
     * @param bid   block ID being the hash
     * @returns a promise that resolves with the block
     */
    async getSkipBlock(bid: Buffer): Promise<SkipBlock> {
        const req = new GetSingleBlock({id: bid});

        const block = await this.conn.send<SkipBlock>(req, SkipBlock);
        if (!block.computeHash().equals(block.hash)) {
            throw new Error("invalid block: hash does not match");
        }

        return block;
    }

    /**
     * Get the block by its index and the genesis block ID
     *
     * @param genesis   Genesis block ID
     * @param index     Index of the block
     * @returns a promise that resolves with the block, or reject with an error
     */
    async getSkipBlockByIndex(genesis: Buffer, index: number): Promise<GetSingleBlockByIndexReply> {
        const req = new GetSingleBlockByIndex({genesis, index});

        const reply = await this.conn.send<GetSingleBlockByIndexReply>(req, GetSingleBlockByIndexReply);
        if (!reply.skipblock.computeHash().equals(reply.skipblock.hash)) {
            throw new Error("invalid block: hash does not match");
        }

        return reply;
    }

    /**
     * Get the list of known skipchains
     *
     * @returns a promise that resolves with the list of skipchain IDs
     */
    async getAllSkipChainIDs(): Promise<Buffer[]> {
        const req = new GetAllSkipChainIDs();

        const ret = await this.conn.send<GetAllSkipChainIDsReply>(req, GetAllSkipChainIDsReply);

        return ret.skipChainIDs.map((id) => Buffer.from(id));
    }

    /**
     * Get the shortest path to the more recent block starting from latestID
     *
     * @param latestID  ID of the block
     * @param verify    Verify the integrity of the chain when true
     * @returns a promise that resolves with the list of blocks
     */
    async getUpdateChain(latestID: Buffer, verify = true): Promise<SkipBlock[]> {
        const req = new GetUpdateChain({latestID});
        const ret = await this.conn.send<GetUpdateChainReply>(req, GetUpdateChainReply);
        const blocks = ret.update;

        const last = blocks[blocks.length - 1];
        if (last && last.forwardLinks.length > 0) {
            // more blocks exist but typically the roster has changed
            const rpc = new SkipchainRPC(last.roster);
            const more = await rpc.getUpdateChain(last.hash, verify);

            blocks.splice(-1, 1, ...more);
        }

        if (verify) {
            const err = this.verifyChain(blocks, latestID);
            if (err) {
                throw new Error(`invalid chain received: ${err.message}`);
            }
        }

        return blocks;
    }

    /**
     * Get the latest known block of the skipchain. It will follow the forward
     * links as much as possible and it is resistant to roster changes.
     *
     * @param latestID  the current latest block
     * @param verify    Verify the integrity of the chain
     * @returns a promise that resolves with the block, or reject with an error
     */
    async getLatestBlock(latestID: Buffer, verify = true): Promise<SkipBlock> {
        const blocks = await this.getUpdateChain(latestID, verify);

        return blocks.pop();
    }

    /**
     * Check the given chain of blocks to insure the integrity of the
     * chain by following the forward links and verifying the signatures
     *
     * @param blocks    the chain to check
     * @param firstID   optional parameter to check the first block identity
     * @returns null for a correct chain or a detailed error otherwise
     */
    verifyChain(blocks: SkipBlock[], firstID?: Buffer): Error {
        if (blocks.length === 0) {
            // expect to have blocks
            return new Error("no block returned in the chain");
        }

        if (firstID && !blocks[0].computeHash().equals(firstID)) {
            // expect the first block to be a particular block
            return new Error("the first ID is not the one we have");
        }

        for (let i = 1; i < blocks.length; i++) {
            const prev = blocks[i - 1];
            const curr = blocks[i];

            if (!curr.computeHash().equals(curr.hash)) {
                return new Error("invalid block hash");
            }

            if (prev.forwardLinks.length === 0) {
                return new Error("no forward link included in the skipblock");
            }

            const link = prev.forwardLinks.find((l) => l.to.equals(curr.hash));
            if (!link) {
                return new Error("no forward link associated with the next block");
            }

            const publics = prev.roster.getServicePublics(SkipchainRPC.serviceName);
            const err = link.verifyWithScheme(publics, prev.signatureScheme);
            if (err) {
                return new Error(`invalid link: ${err.message}`);
            }
        }

        return null;
    }
}
