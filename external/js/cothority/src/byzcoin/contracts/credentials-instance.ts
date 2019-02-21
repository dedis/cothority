import { Message, Properties } from "protobufjs/light";
import Signer from "../../darc/signer";
import { EMPTY_BUFFER, registerMessage } from "../../protobuf";
import ByzCoinRPC from "../byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../client-transaction";
import Instance, { InstanceID } from "../instance";

export default class CredentialsInstance {
    static readonly contractID = "credential";

    /**
     * Get an existing credential instance using its instance ID by fetching
     * the proof.
     * @param bc    the byzcoin RPC
     * @param iid   the instance ID
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<CredentialsInstance> {
        return new CredentialsInstance(bc, await Instance.fromByzCoin(bc, iid));
    }

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
        this.instance = await Instance.fromByzCoin(this.rpc, this.instance.id);
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
        const cred = this.credential.credentials.find((c) => c.name === credential);
        if (!cred) {
            return null;
        }
        const att = cred.attributes.find((a) => a.name === attribute);
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
        let cred = this.credential.credentials.find((c) => c.name === credential);
        if (!cred) {
            cred = new Credential({ name: credential, attributes: [new Attribute({ name: attribute, value })] });
            this.credential.credentials.push(cred);
        } else {
            const idx = cred.attributes.findIndex((a) => a.name === attribute);
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
}

/**
 * Data of a credential instance. It contains none, one or multiple
 * credentials.
 */
export class CredentialStruct extends Message<CredentialStruct> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.CredentialStruct", CredentialStruct, Credential);
    }

    readonly credentials: Credential[];

    constructor(properties?: Properties<CredentialStruct>) {
        super(properties);

        this.credentials = this.credentials || [];
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
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.Credential", Credential, Attribute);
    }

    readonly name: string;
    readonly attributes: Attribute[];

    constructor(props?: Properties<Credential>) {
        super(props);

        this.attributes = this.attributes || [];
    }
}

/**
 * Attribute of a credential
 */
export class Attribute extends Message<Attribute> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.Attribute", Attribute);
    }

    readonly name: string;
    readonly value: Buffer;

    constructor(props?: Properties<Attribute>) {
        super(props);

        this.value = Buffer.from(this.value || EMPTY_BUFFER);
    }
}

<<<<<<< 12789cc9cbe6d17b05d7992cba3358792e452833
/* TODO: remove comment after personhood.online is merged
CredentialStruct.register();
Credential.register();
Attribute.register();
*/
=======
registerMessage("personhood.CredentialStruct", CredentialStruct);
registerMessage("personhood.Credential", Credential);
registerMessage("personhood.Attribute", Attribute);
>>>>>>> Adding latest personhood service and contracts
