import { Message, Properties } from "protobufjs";
import { Roster } from "../network/proto";
import Proof from "./proof";

export default class ChainConfig extends Message<ChainConfig> {
    readonly roster: Roster;
    private blockinterval: number;
    private maxblocksize: number;

    public static fromProof(proof: Proof): ChainConfig {
        return ChainConfig.decode(proof.stateChangeBody.value);
    }

    constructor(properties?: Properties<ChainConfig>) {
        super(properties);

        if (!properties) {
            return;
        }

        const { blockInterval, maxBlockSize } = properties;

        this.blockinterval = this.blockinterval || blockInterval;
        this.maxblocksize = this.maxblocksize || maxBlockSize;
    }

    get blockInterval(): number {
        return this.blockinterval;
    }

    set blockInterval(v: number) {
        this.blockinterval = v;
    }

    get maxBlockSize(): number {
        return this.maxblocksize;
    }

    set maxBlockSize(v: number) {
        this.maxblocksize = v;
    }
}
