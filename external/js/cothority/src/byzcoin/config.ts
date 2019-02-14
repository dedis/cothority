import { Message, Properties } from "protobufjs";
import { Roster } from "../network/proto";
import { registerMessage } from "../protobuf";
import Proof from "./proof";

export default class ChainConfig extends Message<ChainConfig> {
    static fromProof(proof: Proof): ChainConfig {
        return ChainConfig.decode(proof.stateChangeBody.value);
    }

    readonly roster: Roster;
    private blockinterval: Long;
    private maxblocksize: number;

    constructor(properties?: Properties<ChainConfig>) {
        super(properties);

        if (!properties) {
            return;
        }

        const { blockInterval, maxBlockSize } = properties;

        this.blockinterval = this.blockinterval || blockInterval;
        this.maxblocksize = this.maxblocksize || maxBlockSize;
    }

    get blockInterval(): Long {
        return this.blockinterval;
    }

    set blockInterval(v: Long) {
        this.blockinterval = v;
    }

    get maxBlockSize(): number {
        return this.maxblocksize;
    }

    set maxBlockSize(v: number) {
        this.maxblocksize = v;
    }
}

registerMessage("byzcoin.ChainConfig", ChainConfig);
