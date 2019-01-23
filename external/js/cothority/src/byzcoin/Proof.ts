import {ForwardLink} from "~/lib/cothority/skipchain/Structures";
import {SkipBlock} from "~/lib/cothority/skipchain/SkipBlock";
import {objToProto, Root} from "~/lib/cothority/protobuf/Root";
import {Log} from "~/lib/Log";
import {InstanceID} from "~/lib/cothority/byzcoin/ClientTransaction";

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
export class Proof {
    inclusionproof: InclusionProof;
    latest: SkipBlock;
    links: ForwardLink[];
    stateChangeBody: StateChangeBody;
    matches: boolean;
    reqIID: InstanceID;

    /**
     * The constructor is only called from the `fromProto` method. It will interpret the data of the instance
     * in case of a proof of existence.
     *
     * @param p the object returned form the protobuf decoder
     * @param iid the instanceID that this proof represents
     */
    constructor(p: any, iid: InstanceID) {
        this.reqIID = iid;
        if (p == null){
            // This is for testing
            return;
        }
        this.inclusionproof = new InclusionProof(p.inclusionproof);
        this.latest = p.latest;
        this.links = p.links;
        this.matches = iid.iid.equals(this.inclusionproof.key);
        if (this.matches){
            this.stateChangeBody = StateChangeBody.fromProto(this.inclusionproof.value);
        }
    }

    /**
     * Returns the instanceID this proof has been generated for.
     */
    get requestedIID(): InstanceID {
        return this.reqIID;
    }

    /**
     * Returns the contractID this proof represents. Throws an error if it
     * is a proof of absence.
     */
    get contractID(): string {
        if (!this.matches){
            Log.error("not a matching proof");
            return "";
        }
        return this.stateChangeBody.contractID;
    }

    /**
     * @return {InstanceID} the darcID responsible for the instanceID this proof represents.
     * @throws an error if it is a proof of absence.
     */
    get darcID(): InstanceID {
        if (!this.matches){
            Log.error("not a matching proof");
            return null;
        }
        return new InstanceID(this.stateChangeBody.darcID);
    }

    /**
     * The value of the instance is different from the value stored in the global state.
     *
     * @return {Buffer} the value of the instance this proof represents.
     * @throws an error if it is a proof of absence.
     */
    get value(): Buffer {
        if (!this.matches){
            Log.error("not a matching proof");
            return null;
        }
        return this.stateChangeBody.value;
    }

    /**
     * @return {number} the version of the instance this proof represents.
     * @throws an error if it is a proof of absence.
     */
    get version(): number {
        if (!this.matches){
            Log.error("not a matching proof");
            return -1;
        }
        return this.stateChangeBody.version;
    }

    /**
     * Checks whether it is a proof of existence and throws an error if not.
     * @param {string=""} cid optional parameter to check for this type of contract
     * @throws an error if it doesn't match.
     */
    matchOrFail(cid: string = ""): Promise<boolean> {
        // Differentiate the two cases for easier debugging if something fails.
        if (!this.matches) {
            return Promise.reject("cannot get instanceID of non-matching proof");
        }
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
        return this.matches && this.stateChangeBody.contractID == cid;
    }

    /**
     * @return {Buffer} a protobuf representation of this proof.
     */
    toProto(): Buffer {
        return objToProto(this, "Proof");
    }

    /**
     * @return {string} a nicely formatted representation of the proof.
     */
    toString(): string {
        if (!this.matches){
            return "A non-matching proof for " + this.reqIID.iid.toString('hex');
        }
        let str = "Proof for contractID(" + this.contractID + ") for "
            + this.inclusionproof.key.toString('hex');
        return str;
    }

    /**
     * Static method to create a proof from a protobuf representation.
     * @param buf the buffer received from the server (or stored on disk)
     * @param iid the instanceID this proof should represent
     */
    static fromProto(buf: Buffer, iid: InstanceID): Proof {
        return new Proof(Root.lookup('byzcoin.Proof').decode(buf), iid);
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
export class StateChangeBody {
    stateAction: number;
    contractID: string;
    value: Buffer;
    version: number;
    darcID: Buffer;

    constructor(obj: any) {
        this.stateAction = obj.stateAction;
        this.contractID = Buffer.from(obj.contractid).toString();
        this.value = Buffer.from(obj.value);
        this.version = obj.version;
        this.darcID = Buffer.from(obj.darcid);
    }

    static fromProto(buf: Buffer): StateChangeBody {
        return new StateChangeBody(Root.lookup("byzcoin.StateChangeBody").decode(buf));
    }
}