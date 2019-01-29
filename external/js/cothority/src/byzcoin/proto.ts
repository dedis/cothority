import { Message } from "protobufjs";
import { ClientTransaction } from "./ClientTransaction";
import { Roster } from "../network/proto";
import { Darc } from "../darc/Darc";
import { SkipBlock } from "../skipchain/skipblock";
import { Proof, InclusionProof, StateChangeBody } from "./Proof";
import { registerMessage } from "../protobuf";

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

export class CreateGenesisBlockResponse extends Message<CreateGenesisBlockResponse> {
    readonly version: number;
    readonly skipblock: SkipBlock;
}

export class GetProof extends Message<GetProof> {
    readonly version: number;
    readonly key: Buffer;
    readonly id: Buffer;
}

export class GetProofResponse extends Message<GetProofResponse> {
    readonly version: number;
    readonly proof: Proof;
}

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

export class AddTxResponse extends Message<AddTxResponse> {
    readonly version: number;
}

export class GetSignerCounters extends Message<GetSignerCounters> {
    readonly signerids: string[];
    readonly skipchainid: Buffer;
}

export class GetSignerCountersResponse extends Message<GetSignerCountersResponse> {
    readonly counters: Long[];
}

// Add the registration here because the Proof module is optimized
// during compilation and is ignored because we use Proof only as
// a type definition
registerMessage('byzcoin.Proof', Proof);
registerMessage('trie.Proof', InclusionProof);
registerMessage('StateChangeBody', StateChangeBody);

registerMessage('CreateGenesisBlock', CreateGenesisBlock);
registerMessage('CreateGenesisBlockResponse', CreateGenesisBlockResponse);
registerMessage('GetProof', GetProof);
registerMessage('GetProofResponse', GetProofResponse);
registerMessage('AddTxRequest', AddTxRequest);
registerMessage('AddTxResponse', AddTxResponse);
registerMessage('GetSignerCounters', GetSignerCounters);
registerMessage('GetSignerCountersResponse', GetSignerCountersResponse);
