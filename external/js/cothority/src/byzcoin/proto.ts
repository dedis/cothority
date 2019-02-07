import { Message, Properties } from "protobufjs";
import Darc from "../darc/darc";
import { Roster } from "../network/proto";
import { registerMessage } from "../protobuf";
import { SkipBlock } from "../skipchain/skipblock";
import ClientTransaction from "./client-transaction";
import Proof, { registerProofMessages } from "./proof";

/**
 * Request to create a byzcoin skipchain
 */
export class CreateGenesisBlock extends Message<CreateGenesisBlock> {
    readonly version: number;
    readonly roster: Roster;
    private genesisdarc: Darc;
    private blockinterval: Long;
    private maxblocksize: number;

    constructor(properties?: Properties<CreateGenesisBlock>) {
        const props: { [k: string]: any } = {};
        if (properties) {
            // convert camel-cased fields to protobuf fields
            Object.keys(properties).forEach((key) => {
                // @ts-ignore
                props[key.toLowerCase()] = properties[key];
            });
        }

        super(props);
    }

    /**
     * Getter for the genesis darc
     * @returns the genesis darc
     */
    get genesisDarc(): Darc {
        return this.genesisdarc;
    }

    /**
     * Getter for the block interval
     * @returns the interval
     */
    get blockInterval(): Long {
        return this.blockinterval;
    }

    /**
     * Getter for the block maximum size
     * @returns the maximum size
     */
    get maxBlockSize(): number {
        return this.maxblocksize;
    }
}

/**
 * Response of a request to create byzcoin skipchain
 */
export class CreateGenesisBlockResponse extends Message<CreateGenesisBlockResponse> {
    readonly version: number;
    readonly skipblock: SkipBlock;
}

/**
 * Request to get the proof of presence/absence of a given key
 */
export class GetProof extends Message<GetProof> {
    readonly version: number;
    readonly key: Buffer;
    readonly id: Buffer;
}

/**
 * Response of a proof request
 */
export class GetProofResponse extends Message<GetProofResponse> {
    readonly version: number;
    readonly proof: Proof;
}

/**
 * Request to add a transaction
 */
export class AddTxRequest extends Message<AddTxRequest> {
    readonly version: number;
    readonly transaction: ClientTransaction;
    readonly inclusionwait: number;
    private skipchainid: Buffer;

    constructor(properties?: Properties<AddTxRequest>) {
        const props: { [k: string]: any } = {};

        if (properties) {
            // convert camel-cased fields to protobuf fields
            Object.keys(properties).forEach((key) => {
                // @ts-ignore
                props[key.toLowerCase()] = properties[key];
            });
        }

        super(props);
    }

    /**
     * Getter for the skipchain id
     * @returns the id
     */
    get skipchainID(): Buffer {
        return this.skipchainid;
    }
}

/**
 * Response of a request to add a transaction
 */
export class AddTxResponse extends Message<AddTxResponse> {
    readonly version: number;
}

/**
 * Request to get the current counters for given signers
 */
export class GetSignerCounters extends Message<GetSignerCounters> {
    readonly signerids: string[];
    readonly skipchainid: Buffer;
}

/**
 * Response of a counter request in the same order as the signers array
 */
export class GetSignerCountersResponse extends Message<GetSignerCountersResponse> {
    readonly counters: Long[];
}

// Add the registration here because the Proof module is optimized
// during compilation and is ignored because we use Proof only as
// a type definition
registerProofMessages();

registerMessage("CreateGenesisBlock", CreateGenesisBlock);
registerMessage("CreateGenesisBlockResponse", CreateGenesisBlockResponse);
registerMessage("GetProof", GetProof);
registerMessage("GetProofResponse", GetProofResponse);
registerMessage("AddTxRequest", AddTxRequest);
registerMessage("AddTxResponse", AddTxResponse);
registerMessage("GetSignerCounters", GetSignerCounters);
registerMessage("GetSignerCountersResponse", GetSignerCountersResponse);
