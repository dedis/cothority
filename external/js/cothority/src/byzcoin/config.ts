import { Message, Properties } from "protobufjs/light";
import { Roster } from "../network/proto";
import { registerMessage } from "../protobuf";
import Proof from "./proof";

export default class ChainConfig extends Message<ChainConfig> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.ChainConfig", ChainConfig);
    }

    /**
     * Create a chain configuration from a known instance
     * @param proof The proof for the instance
     */
    static fromProof(proof: Proof): ChainConfig {
        return ChainConfig.decode(proof.stateChangeBody.value);
    }

    readonly roster: Roster;
    readonly blockInterval: Long;
    readonly maxBlockSize: number;

    constructor(properties?: Properties<ChainConfig>) {
        super(properties);

        /* Protobuf aliases */

        Object.defineProperty(this, "blockinterval", {
            get(): Long {
                return this.blockInterval;
            },
            set(value: Long) {
                this.blockInterval = value;
            },
        });

        Object.defineProperty(this, "maxblocksize", {
            get(): number {
                return this.maxBlockSize;
            },
            set(value: number) {
                this.maxBlockSize = value;
            },
        });
    }
}

ChainConfig.register();
