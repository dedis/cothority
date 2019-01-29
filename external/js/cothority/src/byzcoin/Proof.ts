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

    get stateChangeBody(): StateChangeBody {
        return StateChangeBody.decode(this.inclusionproof.value);
    }

    /**
     * Returns the contractID this proof represents. Throws an error if it
     * is a proof of absence.
     */
    get contractID(): Buffer {
        return this.stateChangeBody.contractID;
    }

    /**
     * @return {InstanceID} the darcID responsible for the instanceID this proof represents.
     * @throws an error if it is a proof of absence.
     */
    get darcID(): Buffer {
        return this.stateChangeBody.darcID;
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
    get version(): Long {
        return this.stateChangeBody.version;
    }

    get key(): Buffer {
        return this.inclusionproof.key;
    }

    exists(key: Buffer): boolean {
        // TODO
        return true;
    }

    /**
     * @param cid {string} contractID to check
     * @return {boolean} true if it is a proof of existence and the given type of contract matches.
     */
    matchContract(cid: string): boolean {
        return this.stateChangeBody.contractID.toString() == cid;
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
export class InclusionProof extends Message<InclusionProof> {
    interiors: [];
    leaf: any;
    empty: any;
    nonce: Buffer;
    nohashkey: boolean;

    /**
     * @param {InstanceID} iid the instanceID this proof should represent
     * @return {boolean} true if it is a proof of existence.
     */
    matches(iid: Buffer): boolean {
        return Buffer.from(this.leaf.key).equals(iid);
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
export class StateChangeBody extends Message<StateChangeBody> {
    readonly stateaction: number;
    readonly contractid: Buffer;
    readonly value: Buffer;
    readonly version: Long;
    readonly darcid: Buffer;

    get contractID(): Buffer {
        return this.contractid;
    }

    get darcID(): Buffer {
        return this.darcid;
    }
}
