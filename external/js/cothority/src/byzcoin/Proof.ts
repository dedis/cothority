import { Log } from "../log";
import { InstanceID } from "./ClientTransaction";
import { SkipBlock, ForwardLink } from "../skipchain/skipblock";
import { Message } from "protobufjs";

/**
 * The proof class represents a proof that a given instance with its data is either present or absent in the global
 * state. It does this by proving three different things:
 *
 * 1. that there is a valid chain of blocks from the genesis to the latest block
 * 2. a copy of the latest block to get the root hash of the global state trie
 * 3. an inclusion proof against the root hash that can be positive (element is there) or negative (absence of element)
 *
 * As the element that is proven is always an instance, this class also has convenience methods to access the
 * instance data in case it is a proof of existence. For absence proofs, these methods will throw an error.
 */
export class Proof extends Message<Proof> {
    inclusionproof: InclusionProof;
    latest: SkipBlock;
    links: ForwardLink[];

    get stateChangeBody(): StateChange {
        return StateChange.decode(this.inclusionproof.value);
    }

    /**
     * Returns the contractID this proof represents. Throws an error if it
     * is a proof of absence.
     */
    get contractID(): string {
        return this.stateChangeBody.contractID;
    }

    /**
     * @return {InstanceID} the darcID responsible for the instanceID this proof represents.
     * @throws an error if it is a proof of absence.
     */
    get darcID(): InstanceID {
        return new InstanceID(this.stateChangeBody.darcID);
    }

    /**
     * The value of the instance is different from the value stored in the global state.
     *
     * @return {Buffer} the value of the instance this proof represents.
     * @throws an error if it is a proof of absence.
     */
    get value(): Buffer {
        return this.stateChangeBody.value;
    }

    /**
     * @return {number} the version of the instance this proof represents.
     * @throws an error if it is a proof of absence.
     */
    get version(): number {
        return this.stateChangeBody.version;
    }

    /**
     * Checks whether it is a proof of existence and throws an error if not.
     * @param {string=""} cid optional parameter to check for this type of contract
     * @throws an error if it doesn't match.
     */
    matchOrFail(cid: string = ""): Promise<boolean> {
        if (!this.matchContract(cid)) {
            return Promise.reject("proof for '" + this.contractID + "' instead of '" + cid + "'");
        }
        return Promise.resolve(true);
    }

    /**
     * @param cid {string} contractID to check
     * @return {boolean} true if it is a proof of existence and the given type of contract matches.
     */
    matchContract(cid: string): boolean {
        return this.stateChangeBody.contractID == cid;
    }

    /**
     * @return {string} a nicely formatted representation of the proof.
     */
    toString(): string {
        let str = "Proof for contractID(" + this.contractID + ") for "
            + this.inclusionproof.key.toString('hex');
        return str;
    }
}

/**
 * InclusionProof represents the proof that an instance is present or not in the global state trie.
 */
export class InclusionProof {
    interiors: [];
    leaf: any;
    empty: any;
    nonce: Buffer;
    nohashkey: boolean;

    /**
     * Constructs a new inclusionproof given an object from a decoded protobuf.
     * @param ip
     */
    constructor(ip: any) {
        this.interiors = ip.interiors;
        this.leaf = ip.leaf;
        this.empty = ip.empty;
        this.nonce = ip.nonce;
        this.nohashkey = ip.nohashkey;
    }

    /**
     * @param {InstanceID} iid the instanceID this proof should represent
     * @return {boolean} true if it is a proof of existence.
     */
    matches(iid: InstanceID): boolean {
        return Buffer.from(this.leaf.key).equals(iid.iid);
    }

    /**
     * @return {Buffer} the key in the leaf for this inclusionProof. This is not the same as the key this proof has
     * been created for!
     */
    get key(): Buffer {
        return Buffer.from(this.leaf.key);
    }

    /**
     * @return {Buffer} the value stored in the instance. The value of an instance holds the contractID, darcID,
     * version and the data of the instance.
     */
    get value(): Buffer {
        return Buffer.from(this.leaf.value);
    }

    /**
     * @return {Buffer[]} an array of length two for the key and the value. For a proof of absence, the key is not
     * the same as the requested key.
     */
    get keyValue(): Buffer[] {
        return [Buffer.from(this.leaf.key), Buffer.from(this.leaf.value)];
    }
}

/**
 * StateChangeBody represents the
 */
export class StateChange extends Message<StateChange> {
    readonly stateaction: number;
    readonly instanceid: number;
    readonly contractid: string;
    readonly value: Buffer;
    readonly darcid: Buffer;
    readonly version: number;

    get contractID(): string {
        return this.contractid;
    }

    get darcID(): Buffer {
        return this.darcid;
    }
}