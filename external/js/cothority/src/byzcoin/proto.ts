import { Message } from "protobufjs";
import ClientTransaction from "./client-transaction";
import { Roster } from "../network/proto";
import Darc from "../darc/darc";
import { SkipBlock } from "../skipchain/skipblock";
import Proof, { registerProofMessages } from "./proof";
import { registerMessage } from "../protobuf";

/**
 * Request to create a byzcoin skipchain
 */
export class CreateGenesisBlock extends Message<CreateGenesisBlock> {
    readonly version: number;
    readonly roster: Roster;
    private genesisdarc: Darc;
    private blockinterval: number;
    private maxblocksize: number;

    get genesisDarc(): Darc {
        return this.genesisdarc;
    }

    set genesisDarc(darc: Darc) {
        this.genesisdarc = darc;
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
        this.maxBlockSize = v;
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
    private skipchainid: Buffer;
    readonly transaction: ClientTransaction;
    readonly inclusionwait: number;

    get skipchainID(): Buffer {
        return this.skipchainid;
    }

    set skipchainID(id: Buffer) {
        this.skipchainid = id;
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

registerMessage('CreateGenesisBlock', CreateGenesisBlock);
registerMessage('CreateGenesisBlockResponse', CreateGenesisBlockResponse);
registerMessage('GetProof', GetProof);
registerMessage('GetProofResponse', GetProofResponse);
registerMessage('AddTxRequest', AddTxRequest);
registerMessage('AddTxResponse', AddTxResponse);
registerMessage('GetSignerCounters', GetSignerCounters);
registerMessage('GetSignerCountersResponse', GetSignerCountersResponse);