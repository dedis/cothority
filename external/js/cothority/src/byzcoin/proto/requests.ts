import Long from "long";
import { Message, Properties } from "protobufjs/light";
import Darc from "../../darc/darc";
import { Roster } from "../../network/proto";
import { registerMessage } from "../../protobuf";
import { ForwardLink, SkipBlock } from "../../skipchain/skipblock";
import { ClientTransaction } from "../index";
import Proof, { InclusionProof, StateChangeBody } from "../proof";

/**
 * Request to create a byzcoin skipchain
 */
export class CreateGenesisBlock extends Message<CreateGenesisBlock> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("CreateGenesisBlock", CreateGenesisBlock, Roster, Darc);
    }

    readonly version: number;
    readonly roster: Roster;
    readonly genesisDarc: Darc;
    readonly blockInterval: Long;
    readonly maxBlockSize: number;
    readonly darcContractIDs: string[];

    constructor(props?: Properties<CreateGenesisBlock>) {
        super(props);

        this.darcContractIDs = this.darcContractIDs || [];

        /* Protobuf aliases */

        Object.defineProperty(this, "genesisdarc", {
            get(): Darc {
                return this.genesisDarc;
            },
            set(value: Darc) {
                this.genesisDarc = value;
            },
        });

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

        Object.defineProperty(this, "darccontractids", {
            get(): string[] {
                return this.darcContractIDs;
            },
            set(value: string[]) {
                this.darcContractIDs = value;
            },
        });
    }
}

/**
 * Response of a request to create byzcoin skipchain
 */
export class CreateGenesisBlockResponse extends Message<CreateGenesisBlockResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("CreateGenesisBlockResponse", CreateGenesisBlockResponse, SkipBlock);
    }

    readonly version: number;
    readonly skipblock: SkipBlock;
}

/**
 * Request to get the proof of presence/absence of a given key
 */
export class GetProof extends Message<GetProof> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetProof", GetProof);
    }

    readonly version: number;
    readonly key: Buffer;
    readonly id: Buffer;
}

/**
 * Response of a proof request
 */
export class GetProofResponse extends Message<GetProofResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetProofResponse", GetProofResponse, Proof);
    }

    readonly version: number;
    readonly proof: Proof;
}

/**
 * Request to add a transaction
 */
export class AddTxRequest extends Message<AddTxRequest> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("AddTxRequest", AddTxRequest, ClientTransaction);
    }

    readonly version: number;
    readonly transaction: ClientTransaction;
    readonly inclusionwait: number;
    readonly skipchainID: Buffer;

    constructor(props?: Properties<AddTxRequest>) {
        super(props);

        /* Protobuf aliases */

        Object.defineProperty(this, "skipchainid", {
            get(): Buffer {
                return this.skipchainID;
            },
            set(value: Buffer) {
                this.skipchainID = value;
            },
        });
    }
}

/**
 * Response of a request to add a transaction
 */
export class AddTxResponse extends Message<AddTxResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("AddTxResponse", AddTxResponse);
    }

    readonly version: number;
    readonly error: string;
}

/**
 * Request to get the current counters for given signers
 */
export class GetSignerCounters extends Message<GetSignerCounters> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetSignerCounters", GetSignerCounters);
    }

    readonly signerIDs: string[];
    readonly skipchainID: Buffer;

    constructor(props?: Properties<GetSignerCounters>) {
        super(props);

        this.signerIDs = this.signerIDs || [];

        /* Protobuf aliases */

        Object.defineProperty(this, "signerids", {
            get(): string[] {
                return this.signerIDs;
            },
            set(value: string[]) {
                this.signerIDs = value;
            },
        });

        Object.defineProperty(this, "skipchainid", {
            get(): Buffer {
                return this.skipchainID;
            },
            set(value: Buffer) {
                this.skipchainID = value;
            },
        });
    }
}

/**
 * Response of a counter request in the same order as the signers array
 */
export class GetSignerCountersResponse extends Message<GetSignerCountersResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetSignerCountersResponse", GetSignerCountersResponse);
    }

    readonly counters: Long[];

    constructor(props?: Properties<GetSignerCountersResponse>) {
        super(props);

        this.counters = this.counters || [];
    }
}

/**
 * Holds an instanceID and it's latest known version.
 */
export class IDVersion extends Message<IDVersion> {
    static register() {
        registerMessage("IDVersion", IDVersion);
    }

    readonly id: Buffer;
    readonly version: Long;
}

/**
 * Requests a set of proofs that will be returned in a single message.
 * The reply will only hold proofs for instances that have a higher version than the ones given here.
 */
export class GetUpdatesRequest extends Message<GetUpdatesRequest> {
    // send all instances with version 0, even those that are not updated.
    static readonly sendVersion0 = Long.fromNumber(1);
    // send proofs for missing instances. If not present, missing instances are ignored.
    static readonly sendMissingProofs = Long.fromNumber(2);

    static register() {
        registerMessage("GetUpdatesRequest", GetUpdatesRequest, IDVersion);
    }

    readonly instances: IDVersion[];
    readonly flags: Long;
    readonly latestblockid: Buffer;
    readonly skipchainid: Buffer;
}

/**
 * The requested proofs
 */
export class GetUpdatesReply extends Message<GetUpdatesReply> {
    static register() {
        registerMessage("GetUpdatesReply", GetUpdatesReply, InclusionProof, ForwardLink, SkipBlock);
    }

    readonly proofs: InclusionProof[];
    readonly links: ForwardLink[];
    readonly latest: SkipBlock;
}

/**
 * Request information about a specific version of an instance. The node might not have this version in its cache,
 * so it might return an error.
 */
export class GetInstanceVersion extends Message<GetInstanceVersion> {
    static register() {
        registerMessage("GetInstanceVersion", GetInstanceVersion);
    }

    readonly skipchainid: Buffer;
    readonly instanceid: Buffer;
    readonly version: Long;
}

/**
 * Request the latest version of this instance. The difference with this and GetProof is that it doesn't return a
 * full proof, but it includes the block where this instance has been created or modified.
 */
export class GetLastInstanceVersion extends Message<GetLastInstanceVersion> {
    static register() {
        registerMessage("GetLastInstanceVersion", GetLastInstanceVersion);
    }

    readonly skipchainid: Buffer;
    readonly instanceid: Buffer;
}

/**
 * Returns the StateChangeBody and the block index where this instance has been created or modified. Contrary to
 * GetProof, this does not return a full proof that can be verified by the client. For this, the client has to
 * either request the block itself, or ask for a GetProof to get the latest version of the instance.
 */
export class GetInstanceVersionResponse extends Message<GetInstanceVersionResponse> {
    static register() {
        registerMessage("GetInstanceVersionResponse", GetInstanceVersionResponse, StateChangeBody);
    }

    readonly statechange: StateChangeBody;
    readonly blockindex: number;
}

/**
 * Requests all known versions of a given instance. The nodes only hold a certain amount of versions, so this might
 * not go back all the way to the version where the instance has been created.
 */
export class GetAllInstanceVersion extends Message<GetAllInstanceVersion> {
    static register() {
        registerMessage("GetAllInstanceVersion", GetAllInstanceVersion);
    }

    readonly skipchainid: Buffer;
    readonly instanceid: Buffer;
}

/**
 * Reply holding all known versions of this instance.
 */
export class GetAllInstanceVersionResponse extends Message<GetAllInstanceVersionResponse> {
    static register() {
        registerMessage("GetAllInstanceVersionResponse", GetAllInstanceVersionResponse, GetInstanceVersionResponse);
    }

    readonly statechanges: GetInstanceVersionResponse[];
}

CreateGenesisBlock.register();
CreateGenesisBlockResponse.register();
GetProof.register();
GetProofResponse.register();
AddTxRequest.register();
AddTxResponse.register();
GetSignerCounters.register();
GetSignerCountersResponse.register();
GetUpdatesRequest.register();
GetUpdatesReply.register();
GetInstanceVersion.register();
GetLastInstanceVersion.register();
GetInstanceVersionResponse.register();
GetAllInstanceVersion.register();
GetAllInstanceVersionResponse.register();
