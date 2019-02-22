import { Message } from "protobufjs/light";
import { registerMessage } from "../../protobuf";

/**
 * ByzCoin metadata
 */
export default class DataHeader extends Message<DataHeader> {
    readonly trieroot: Buffer;
    readonly clienttransactionhash: Buffer;
    readonly statechangehash: Buffer;
    readonly timestamp: Long;

    /**
     * Getter for the trie root
     *
     * @returns the trie root
     */
    get trieRoot(): Buffer {
        return this.trieroot;
    }

    /**
     * Getter for the client transactions' hash
     *
     * @returns the hash
     */
    get clientTransactionHash(): Buffer {
        return this.clienttransactionhash;
    }

    /**
     * Getter for the state changes' hash
     *
     * @returns the hash
     */
    get stateChangeHash(): Buffer {
        return this.statechangehash;
    }
}

registerMessage("byzcoin.DataHeader", DataHeader);
