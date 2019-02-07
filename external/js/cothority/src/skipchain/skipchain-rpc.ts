import { Connection, WebSocketConnection } from "../network/connection";
import { Roster } from "../network/proto";
import {
    StoreSkipBlock,
    StoreSkipBlockReply,
    GetSingleBlock,
    GetUpdateChain,
    GetUpdateChainReply,
} from "./proto";
import { SkipBlock } from "./skipblock";
import Logger from "../log";

/**
 * SkipchainRPC provides basic tools to interact with a skipchain
 * with a given roster
 */
export default class SkipchainRPC {
    public static ServiceName = 'Skipchain';

    private roster: Roster;
    private conn: Connection[];

    constructor(roster: Roster) {
        this.roster = roster;
        this.conn = roster.list.map((srvid) => {
            return new WebSocketConnection(srvid.getWebSocketAddress(), SkipchainRPC.ServiceName);
        });
    }

    /**
     * Create a skipchain with a base and a max height
     * @param baseHeight    base height of the skipchain
     * @param maxHeight     maximum height of the skipchain
     * @returns a promise that resolves with the genesis block
     * @throws an error if the request is not successful
     */
    createSkipchain(baseHeight: number = 1, maxHeight: number = 3): Promise<StoreSkipBlockReply> {
        const newBlock = new SkipBlock({ roster: this.roster, maxHeight, baseHeight });
        const req = new StoreSkipBlock({ newBlock });

        return this.conn[0].send(req, StoreSkipBlockReply);
    }

    /**
     * Add a new block to a given skipchain
     * @param gid the genesis ID of the skipchain
     * @param msg the data to include in the block
     * @throws an error if the request is not successful
     */
    addBlock(gid: Buffer, msg: Buffer): Promise<StoreSkipBlockReply> {
        const newBlock = new SkipBlock({ roster: this.roster, data: msg });
        const req = new StoreSkipBlock({
            targetSkipChainID: gid,
            newBlock,
        });

        return this.conn[0].send(req, StoreSkipBlockReply);
    }

    /**
     * Get the block with the given ID
     * @param bid   block ID being the hash
     * @returns a   promise that resolves with the block
     * @throws an error if the request is not successful
     */
    getSkipblock(bid: Buffer): Promise<SkipBlock> {
        const req = new GetSingleBlock({ id: bid });

        return this.conn[0].send(req, SkipBlock);
    }

    /**
     * Get the latest known block of the skipchain. It will follow the forward
     * links as much as possible and it is resistant to roster changes.
     * @param latestID  the current latest block
     * @param roster    use a different roster than the RPC
     * @throws an error if the latest block can't be fetched
     */
    async getLatestBlock(latestID: Buffer, roster?: Roster): Promise<SkipBlock> {
        const req = new GetUpdateChain({ latestID });
        let reply: GetUpdateChainReply;

        for (let i = 0; i < this.conn.length; i++) {
            try {
                reply = await this.conn[i].send(req, GetUpdateChainReply);
            } catch (err) {
                Logger.error(`Failed to reach ${this.conn[i].getURL()}`);
                continue;
            }

            const err = this.verifyChain(reply.update, latestID);
            if (!err) {
                const b = reply.update.pop();

                if (b.forwardLinks.length === 0) {
                    return b;
                } else {
                    // it might happen a conode doesn't have the latest
                    // block stored so we contact the most updated
                    // roster to try to get it
                    return new SkipchainRPC(b.roster).getLatestBlock(b.hash);
                }
            } else {
                Logger.lvl3('Received corrupted skipchain with error:', err);
            }
        }

        // in theory that should not happen as at least the leader has the latest block
        throw new Error('No conode has the latest block');
    }

    /**
     * Check the given chain of blocks to insure the integrity of the
     * chain by following the forward links and verifying the signatures
     * @param blocks    the chain to check
     * @param firstID   optional parameter to check the first block identity
     * @returns null for a correct chain or a detailed error otherwise
     */
    verifyChain(blocks: SkipBlock[], firstID?: Buffer): Error {
        if (blocks.length === 0) {
            // expect to have blocks
            return new Error('No block returned in the chain');
        }

        if (firstID && !blocks[0].hash.equals(firstID)) {
            // expect the first block to be a particular block
            return new Error('the first ID is not the one we have');
        }

        for (let i = 1; i < blocks.length; i++) {
            const prev = blocks[i-1];
            const curr = blocks[i];

            if (prev.forwardLinks.length === 0) {
                return new Error('No forward link included in the skipblock');
            }

            const link = prev.forwardLinks.find(l => l.to.equals(curr.hash));
            if (!link) {
                return new Error('No forward link associated with the next block');
            }

            const err = link.verify(curr.roster.getServicePublics(SkipchainRPC.ServiceName));
            if (err) {
                return err;
            }
        }

        return null;
    }
}
