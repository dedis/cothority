import _ from 'lodash';
import { Message } from "protobufjs";
import { SkipBlock, ForwardLink } from "../skipchain/skipblock";
import { registerMessage } from "../protobuf";
import { createHash } from "crypto";

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
export default class Proof extends Message<Proof> {
    inclusionproof: InclusionProof;
    latest: SkipBlock;
    links: ForwardLink[];

    /**
     * Get the state change stored in the inclusion proof
     * 
     * @returns the state change body
     */
    get stateChangeBody(): StateChangeBody {
        return StateChangeBody.decode(this.inclusionproof.value);
    }

    /**
     * Returns the contractID this proof represents. Throws an error if it
     * is a proof of absence.
     * 
     * @returns the contract ID as a buffer
     */
    get contractID(): Buffer {
        return this.stateChangeBody.contractID;
    }

    /**
     * Get the darc ID of the instance
     * 
     * @returns the darcID responsible for the instanceID this proof represents.
     */
    get darcID(): Buffer {
        return this.stateChangeBody.darcID;
    }

    /**
     * The value of the instance is different from the value stored in the global state.
     *
     * @returns the value of the instance this proof represents.
     */
    get value(): Buffer {
        return this.stateChangeBody.value;
    }

    /**
     * Get the version of the instance
     * 
     * @returns the version of the instance this proof represents.
     */
    get version(): Long {
        return this.stateChangeBody.version;
    }

    /**
     * Get the instance ID for the proof
     * 
     * @returns the instance ID as a buffer
     */
    get key(): Buffer {
        return this.inclusionproof.key;
    }

    /**
     * Check the proof is well formed and that the instance ID
     * stored in the leaf exists in the proof
     */
    matches(): boolean {
        if (!this.inclusionproof.leaf) {
            return false;
        }

        return this.exists(this.key);
    }

    /**
     * Check if the key exists in the proof
     * 
     * @returns true when it exists, false otherwise
     * @throws for corrupted proofs
     */
    exists(key: Buffer): boolean {
        if (key.length === 0) {
            throw new Error('key is nil');
        }
        if (this.inclusionproof.interiors.length === 0) {
            throw new Error('no interior node');
        }

        const bits = hashToBits(key);
        let expectedHash = this.inclusionproof.hashInterior(0);

        let i = 0;
        for (; i < this.inclusionproof.interiors.length; i++) {
            if (!expectedHash.equals(this.inclusionproof.hashInterior(i))) {
                throw new Error('invalid interior node');
            }

            if (bits[i]) {
                expectedHash = this.inclusionproof.interiors[i].left;
            } else {
                expectedHash = this.inclusionproof.interiors[i].right;
            }
        }

        if (expectedHash.equals(this.inclusionproof.hashLeaf())) {
            if ( _.difference(bits.slice(0, i), this.inclusionproof.leaf.prefix).length !== 0) {
                throw new Error('invalid prefix in leaf node');
            }

            return this.key.equals(key);
        } else if (expectedHash.equals(this.inclusionproof.hashEmpty())) {
            if (_.difference(bits.slice(0, i), this.inclusionproof.empty.prefix).length !== 0) {
                throw new Error('invalid prefix in empty node');
            }

            return false;
        }

        throw new Error('no corresponding leaf/empty ndoe with respect to the interior node');
    }

    /**
     * @param cid contractID to check
     * @return true if it is a proof of existence and the given type of contract matches.
     */
    matchContract(cid: string): boolean {
        return this.stateChangeBody.contractID.toString() == cid;
    }

    /**
     * @return a nicely formatted representation of the proof.
     */
    toString(): string {
        let str = "Proof for contractID(" + this.contractID + ") for "
            + this.inclusionproof.key.toString('hex');
        return str;
    }
}

/**
 * Get an array of booleans depending on the binary representation
 * of the key
 * 
 * @param key the key to hash
 * @returns an array of booleans matching the key binary value
 */
function hashToBits(key: Buffer): boolean[] {
    const h = createHash('sha256');
    h.update(key);
    const hash = h.digest();

    const bits = new Array(hash.length * 8);
    for (let i = 0; i < bits.length; i++) {
        bits[i] = ((hash[i >> 3] << (i % 8)) & (1 << 7)) > 0;
    }

    return bits;
}

/**
 * Get a buffer from an array of boolean converted in binary
 * 
 * @param bits the array of booleans
 * @returns a buffer of the binary shape
 */
function boolToBuffer(bits: boolean[]): Buffer {
    const buf = Buffer.alloc((bits.length + 7) >> 3, 0);

    for (let i = 0; i < bits.length; i++) {
        if (bits[i]) {
            buf[i >> 3] |= (1 << 7) >> (i % 8);
        }
    }

    return buf;
}

/**
 * Interior node of an inclusion proof
 */
class InteriorNode extends Message<InteriorNode> {
    readonly left: Buffer;
    readonly right: Buffer;
}

/**
 * Empty node of an inclusion proof
 */
class EmptyNode extends Message<EmptyNode> {
    readonly prefix: boolean[];
}

/**
 * Leaf node of an inclusion proof
 */
class LeafNode extends Message<LeafNode> {
    readonly prefix: boolean[];
    readonly key: Buffer;
    readonly value: Buffer;
}

/**
 * InclusionProof represents the proof that an instance is present or not in the global state trie.
 */
class InclusionProof extends Message<InclusionProof> {
    interiors: InteriorNode[];
    leaf: LeafNode;
    empty: EmptyNode;
    nonce: Buffer;

    /**
     * @param {InstanceID} iid the instanceID this proof should represent
     * @return {boolean} true if it is a proof of existence.
     */
    matches(iid: Buffer): boolean {
        return this.leaf.key.equals(iid);
    }

    /**
     * @return {Buffer} the key in the leaf for this inclusionProof. This is not the same as the key this proof has
     * been created for!
     */
    get key(): Buffer {
        return this.leaf.key;
    }

    /**
     * @return {Buffer} the value stored in the instance. The value of an instance holds the contractID, darcID,
     * version and the data of the instance.
     */
    get value(): Buffer {
        return this.leaf.value;
    }

    /**
     * @return {Buffer[]} an array of length two for the key and the value. For a proof of absence, the key is not
     * the same as the requested key.
     */
    get keyValue(): Buffer[] {
        return [this.leaf.key, this.leaf.value];
    }

    /**
     * Get the hash of the interior node at the given index
     * 
     * @param index the index of the interior node
     * @returns the hash as a buffer
     */
    hashInterior(index: number): Buffer {
        const h = createHash('sha256');
        h.update(this.interiors[index].left);
        h.update(this.interiors[index].right);

        return h.digest();
    }

    /**
     * Get the hash of the leaf of the inclusion proof
     * 
     * @returns the hash as a buffer
     */
    hashLeaf(): Buffer {
        const h = createHash('sha256');
        h.update(Buffer.from([3]));
        h.update(this.nonce);

        const prefix = boolToBuffer(this.leaf.prefix);
        h.update(prefix);

        const length = Buffer.allocUnsafe(4);
        length.writeIntLE(this.leaf.prefix.length, 0, 4);
        h.update(length);

        h.update(this.leaf.key);
        h.update(this.leaf.value);

        return h.digest();
    }

    /**
     * Get the hash of the empty node of the inclusion proof
     * 
     * @returns the hash of the empty node
     */
    hashEmpty(): Buffer {
        const h = createHash('sha256');
        h.update(Buffer.from([2]));
        h.update(this.nonce);

        const prefix = boolToBuffer(this.empty.prefix);
        h.update(prefix);

        const length = Buffer.allocUnsafe(4);
        length.writeIntLE(this.empty.prefix.length, 0, 4);
        h.update(length);

        return h.digest();
    }
}

class StateChangeBody extends Message<StateChangeBody> {
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

export function registerProofMessages() {
    registerMessage('byzcoin.Proof', Proof);
    registerMessage('trie.Proof', InclusionProof);
    registerMessage('trie.InteriorNode', InteriorNode);
    registerMessage('trie.LeafNode', LeafNode);
    registerMessage('trie.EmptyNode', EmptyNode);
    registerMessage('StateChangeBody', StateChangeBody);
}
