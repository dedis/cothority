import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../../protobuf";

const EMPTY_BUFFER = Buffer.allocUnsafe(0);

/**
 * ByzCoin metadata
 */
export default class DataHeader extends Message<DataHeader> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.DataHeader", DataHeader);
    }

    readonly trieRoot: Buffer;
    readonly clientTransactionHash: Buffer;
    readonly stateChangeHash: Buffer;
    readonly timestamp: Long;

    constructor(props?: Properties<DataHeader>) {
        super(props);

        this.trieRoot = Buffer.from(this.trieRoot || EMPTY_BUFFER);
        this.clientTransactionHash = Buffer.from(this.clientTransactionHash || EMPTY_BUFFER);
        this.stateChangeHash = Buffer.from(this.stateChangeHash || EMPTY_BUFFER);

        /* Protobuf aliases */

        Object.defineProperty(this, "trieroot", {
            get(): Buffer {
                return this.trieRoot;
            },
            set(value: Buffer) {
                this.trieRoot = value;
            },
        });

        Object.defineProperty(this, "clienttransactionhash", {
            get(): Buffer {
                return this.clientTransactionHash;
            },
            set(value: Buffer) {
                this.clientTransactionHash = value;
            },
        });

        Object.defineProperty(this, "statechangehash", {
            get(): Buffer {
                return this.stateChangeHash;
            },
            set(value: Buffer) {
                this.stateChangeHash = value;
            },
        });
    }
}

DataHeader.register();
