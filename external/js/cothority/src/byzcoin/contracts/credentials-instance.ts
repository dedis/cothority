import { Message, Properties } from "protobufjs";
import Instance, { InstanceID } from "../instance";
import ByzCoinRPC from "../byzcoin-rpc";
import Signer from "../../darc/signer";
import ClientTransaction, { Instruction, Argument } from "../client-transaction";
import Proof from "../proof";
import { registerMessage } from "../../protobuf";

export default class CredentialsInstance {
    static readonly contractID = "credential";

    private rpc: ByzCoinRPC;
    private instance: Instance;
    private credential: CredentialStruct;

    constructor(bc: ByzCoinRPC, inst: Instance) {
        this.rpc = bc;
        this.instance = inst;
        this.credential = CredentialStruct.decode(inst.data);
    }

    /**
     * Getter for the darc ID
     * 
     * @returns the id as a buffer
     */
    get darcID(): InstanceID {
        return this.instance.darcID;
    }

    /**
     * Update the data of the crendetial instance by fetching the proof
     * 
     * @returns a promise resolving with the instance on success, rejecting with
     * the error otherwise
     */
    async update(): Promise<CredentialsInstance> {
        const proof = await this.rpc.getProof(this.instance.id);
        if (!proof.matches()) {
            throw new Error('fail to get a matching proof');
        }

        this.instance = Instance.fromProof(proof);
        this.credential = CredentialStruct.decode(this.instance.data);
        return this;
    }

    /**
     * Get a credential attribute
     * 
     * @param credential    The name of the credential
     * @param attribute     The name of the attribute
     * @returns the value of the attribute if it exists, null otherwise
     */
    getAttribute(credential: string, attribute: string): Buffer {
        let cred = this.credential.credentials.find(c => c.name == credential);
        if (!cred) {
            return null;
        }
        let att = cred.attributes.find(a => a.name == attribute);
        if (!att) {
            return null;
        }
        return att.value;
    }

    /**
     * Set or update a credential attribute by sending a transaction. It will wait
     * for the block inclusion or throw an error if it fails.
     * 
     * @param owner         Signer to use for the transaction
     * @param credential    Name of the credential
     * @param attribute     Name of the attribute
     * @param value         The value to set
     * @returns a promise resolving when the transaction is in a block, or rejecting
     * for an error
     */
    async setAttribute(owner: Signer, credential: string, attribute: string, value: Buffer): Promise<any> {
        let cred = this.credential.credentials.find(c => c.name == credential);
        if (!cred) {
            cred = new Credential({ name: credential, attributes: [new Attribute({ name: attribute, value })] });
            this.credential.credentials.push(cred);
        } else {
            const idx = cred.attributes.findIndex(a => a.name == attribute);
            const attr = new Attribute({ name: attribute, value });
            if (idx === -1) {
                cred.attributes.push(attr);
            } else {
                cred.attributes[idx] = attr;
            }
        }

        const instr = Instruction.createInvoke(
            this.instance.id,
            CredentialsInstance.contractID,
            "update",
            [new Argument({ name: "credential", value: this.credential.toBytes() })],
        );
        await instr.updateCounters(this.rpc, [owner]);

        const ctx = new ClientTransaction({ instructions: [instr] });
        ctx.signWith([owner]);

        await this.rpc.sendTransactionAndWait(ctx);

        return this;
    }

    /**
     * Instantiate the underlying credential instance if the proof is valid
     * @param bc    the byzcoin RPC
     * @param p     the proof
     */
    static async fromProof(bc: ByzCoinRPC, p: Proof): Promise<CredentialsInstance> {
        if (!p.matches()) {
            throw new Error('fail to get a matching proof');
        }

        return new CredentialsInstance(bc, Instance.fromProof(p));
    }

    /**
     * Get an existing credential instance using its instance ID by fetching
     * the proof.
     * @param bc    the byzcoin RPC
     * @param iid   the instance ID
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<CredentialsInstance> {
        return CredentialsInstance.fromProof(bc, await bc.getProof(iid));
    }
}

/**
 * Data of a credential instance. It contains none, one or multiple
 * credentials.
 */
export class CredentialStruct extends Message<CredentialStruct> {
    readonly credentials: Credential[];

    constructor(properties?: Properties<CredentialStruct>) {
        super(properties);

        if (!properties || !properties.credentials) {
            this.credentials = [];
        }
    }

    /**
     * Helper to encode the struct using protobuf
     * @returns encoded struct as a buffer
     */
    toBytes(): Buffer {
        return Buffer.from(CredentialStruct.encode(this).finish());
    }
}

/**
 * A credential has a given name used as a key and one or more attributes
 */
export class Credential extends Message<Credential> {
    readonly name: string;
    readonly attributes: Attribute[];
}

/**
 * Attribute of a credential
 */
export class Attribute extends Message<Attribute> {
    readonly name: string;
    readonly value: Buffer;
}

registerMessage('personhood.CredentialStruct', CredentialStruct);
registerMessage('personhood.Credential', Credential);
registerMessage('personhood.Attribute', Attribute);
