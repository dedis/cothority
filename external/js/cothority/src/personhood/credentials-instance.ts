import { Point } from "@dedis/kyber";
import { createHash, randomBytes } from "crypto";
import { Message, Properties } from "protobufjs/light";
import ByzCoinRPC from "../byzcoin/byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../byzcoin/client-transaction";
import Instance, { InstanceID } from "../byzcoin/instance";
import Signer from "../darc/signer";
import Log from "../log";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";

export default class CredentialsInstance extends Instance {
    static readonly contractID = "credential";
    static readonly commandUpdate = "update";
    static readonly argumentCredential = "credential";
    static readonly argumentCredID = "credentialID";
    static readonly argumentDarcID = "darcIDBuf";

    /**
     * Generate the credential instance ID for a given public key
     *
     * @param buf the public key in marshalBinary form
     * @returns the id as a buffer
     */
    static credentialIID(buf: Buffer): InstanceID {
        const h = createHash("sha256");
        h.update(Buffer.from(CredentialsInstance.contractID));
        h.update(buf);
        return h.digest();
    }

    /**
     * Spawn a new credential instance from a darc
     *
     * @param bc        The RPC to use
     * @param darcID    The darc instance ID
     * @param signers   The list of signers for the transaction
     * @param cred      The credential to store
     * @param credID    Optional - if given, the instanceID will be sha256("credential" | pub)
     * @param credDarcID Optional - if given, replaces the darc stored in the new credential with credDarcID.
     * @returns a promise that resolves with the new instance
     */
    static async spawn(
        bc: ByzCoinRPC,
        darcID: InstanceID,
        signers: Signer[],
        cred: CredentialStruct,
        credID: Buffer = null,
        credDarcID: InstanceID = null,
    ): Promise<CredentialsInstance> {
        const args = [new Argument({name: CredentialsInstance.argumentCredential, value: cred.toBytes()})];
        if (credID) {
            args.push(new Argument({name: CredentialsInstance.argumentCredID, value: credID}));
        }
        if (credDarcID) {
            args.push(new Argument({name: CredentialsInstance.argumentDarcID, value: credDarcID}));
        }
        const inst = Instruction.createSpawn(
            darcID,
            CredentialsInstance.contractID,
            args,
        );
        await inst.updateCounters(bc, signers);

        const ctx = ClientTransaction.make(bc.getProtocolVersion(), inst);
        ctx.signWith([signers]);

        await bc.sendTransactionAndWait(ctx, 10);

        return CredentialsInstance.fromByzcoin(bc, ctx.instructions[0].deriveId());
    }

    /**
     * Create a new credential instance from a darc
     *
     * @param bc        The RPC to use
     * @param darcID    The darc instance ID
     * @param cred      The credential to store
     * @param credID       Optional - if given, the instanceID will be sha256("credential" | credID)
     * @returns a promise that resolves with the new instance
     */
    static create(
        bc: ByzCoinRPC,
        darcID: InstanceID,
        cred: CredentialStruct,
        credID: Buffer = null,
    ): CredentialsInstance {
        if (!credID) {
            credID = randomBytes(32);
        }
        const inst = new Instance({
            contractID: CredentialsInstance.contractID,
            darcID,
            data: cred.toBytes(),
            id: CredentialsInstance.credentialIID(credID),
        });
        return new CredentialsInstance(bc, inst);
    }

    /**
     * Get an existing credential instance using its instance ID by fetching
     * the proof.
     * @param bc    the byzcoin RPC
     * @param iid   the instance ID
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID, waitMatch: number = 0, interval: number = 1000):
        Promise<CredentialsInstance> {
        return new CredentialsInstance(bc, await Instance.fromByzcoin(bc, iid, waitMatch, interval));
    }
    credential: CredentialStruct;

    constructor(private rpc: ByzCoinRPC, inst: Instance) {
        super(inst);
        if (inst.contractID.toString() !== CredentialsInstance.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${CredentialsInstance.contractID}`);
        }
        this.credential = CredentialStruct.decode(inst.data);
    }

    /**
     * Update the data of the crendetial instance by fetching the proof
     *
     * @returns a promise resolving with the instance on success, rejecting with
     * the error otherwise
     */
    async update(): Promise<CredentialsInstance> {
        const inst = await Instance.fromByzcoin(this.rpc, this.id);
        this.data = inst.data;
        this.darcID = inst.darcID;
        this.credential = CredentialStruct.decode(this.data);
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
        return this.credential.getAttribute(credential, attribute);
    }

    /**
     * Set or update a credential attribute locally. The new credential is not sent to
     * the blockchain, for this you need to call sendUpdate.
     *
     * @param credential    Name of the credential
     * @param attribute     Name of the attribute
     * @param value         The value to set
     * @returns a promise resolving when the transaction is in a block, or rejecting
     * for an error
     */
    async setAttribute(credential: string, attribute: string, value: Buffer): Promise<any> {
        return this.credential.setAttribute(credential, attribute, value);
    }

    /**
     * Creates a transaction to update the credential and sends it to ByzCoin.
     *
     * @param owners a list of signers to fulfill the expression of the `invoke:credential.update` rule.
     * @param newCred the new credentialStruct to store in the instance.
     */
    async sendUpdate(owners: Signer[], newCred: CredentialStruct = null): Promise<CredentialsInstance> {
        if (newCred) {
            this.credential = newCred.copy();
        }
        const instr = Instruction.createInvoke(
            this.id,
            CredentialsInstance.contractID,
            CredentialsInstance.commandUpdate,
            [new Argument({name: CredentialsInstance.argumentCredential, value: this.credential.toBytes()})],
        );
        const ctx = ClientTransaction.make(this.rpc.getProtocolVersion(), instr);
        await ctx.updateCountersAndSign(this.rpc, [owners]);

        await this.rpc.sendTransactionAndWait(ctx);

        return this;
    }

    /**
     * Recovers an identity by giving a list of signatures from trusted people.
     *
     * @param pubKey the new public key for the identity. It will be stored as the new expression for the
     * signer-rule.
     * @param signatures a threshold list of signatures on the public key and the instanceID.
     */
    async recoverIdentity(pubKey: Point, signatures: RecoverySignature[]): Promise<any> {
        const sigBuf = Buffer.alloc(RecoverySignature.pubSig * signatures.length);
        signatures.forEach((s, i) => s.signature.copy(sigBuf, RecoverySignature.pubSig * i));
        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            Instruction.createInvoke(
                this.id,
                CredentialsInstance.contractID,
                "recover",
                [
                    new Argument({name: "signatures", value: sigBuf}),
                    new Argument({name: "public", value: pubKey.toProto()}),
                ],
            ),
        );
        await this.rpc.sendTransactionAndWait(ctx);
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

        this.credentials = this.credentials.slice() || [];
    }

    /**
     * Get a credential attribute
     *
     * @param credential    The name of the credential
     * @param attribute     The name of the attribute
     * @returns the value of the attribute if it exists, null otherwise
     */
    getAttribute(credential: string, attribute: string): Buffer {
        const cred = this.credentials.find((c) => c.name === credential);
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
     * getCredential returns the credential with the given name, or null if
     * nothing found.
     * @param credential name of the credential to return
     */
    getCredential(credential: string): Credential {
        return this.credentials.find((c) => c.name === credential);
    }

    /**
     * Overwrites the credential with name 'name' with the given credential.
     * If it doesn't exist, it will be appended to the list.
     *
     * @param name the name of the credential
     * @param cred the credential to store
     */
    setCredential(name: string, cred: Credential) {
        const index = this.credentials.findIndex((c) => c.name === name);
        if (index < 0) {
            this.credentials.push(cred);
        } else {
            this.credentials[index] = cred;
        }
    }

    /**
     * Set or update a credential attribute locally. The update is not sent to the blockchain.
     * For this you need to call CredentialInstance.sendUpdate().
     *
     * @param owner         Signer to use for the transaction
     * @param credential    Name of the credential
     * @param attribute     Name of the attribute
     * @param value         The value to set
     * @returns a promise resolving when the transaction is in a block, or rejecting
     * for an error
     */
    setAttribute(credential: string, attribute: string, value: Buffer) {
        let cred = this.credentials.find((c) => c.name === credential);
        if (!cred) {
            cred = new Credential({name: credential, attributes: [new Attribute({name: attribute, value})]});
            this.credentials.push(cred);
        } else {
            const idx = cred.attributes.findIndex((a) => a.name === attribute);
            const attr = new Attribute({name: attribute, value});
            if (idx === -1) {
                cred.attributes.push(attr);
            } else {
                cred.attributes[idx] = attr;
            }
        }
    }

    /**
     * Removes the attribute from the given credential. If the credential or the
     * attribute doesn't exist, it returns 'undefined', else it returns the
     * content of the deleted attribute.
     *
     * @param credential the name of the credential
     * @param attribute the attribute to be deleted
     */
    deleteAttribute(credential: string, attribute: string): Buffer {
        const cred = this.getCredential(credential);
        if (!cred) {
            return undefined;
        }
        const index = cred.attributes.findIndex((att) => att.name === attribute);
        if (index < 0) {
            return undefined;
        }
        return cred.attributes.splice(index, 1)[0].value;
    }

    /**
     * Copy returns a new CredentialStruct with copies of all internal data.
     */
    copy(): CredentialStruct {
        return CredentialStruct.decode(this.toBytes());
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

    /**
     * Returns a credential with only the given name/key = value stored in it.
     *
     * @param name the name of the attribute
     * @param key the key to store
     * @param value the value that will be stored in the key
     */
    static fromNameAttr(name: string, key: string, value: Buffer): Credential {
        return new Credential({name, attributes: [new Attribute({name: key, value})]});
    }

    readonly name: string;
    readonly attributes: Attribute[];

    constructor(props?: Properties<Credential>) {
        super(props);

        this.attributes = this.attributes.slice() || [];
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

export class RecoverySignature {
    static readonly sig = 64;
    static readonly pub = 32;
    static readonly credIID = 32;
    static readonly version = 8;
    static readonly pubSig = RecoverySignature.pub + RecoverySignature.sig;
    static readonly msgBuf = RecoverySignature.credIID + RecoverySignature.pub + RecoverySignature.version;

    constructor(public credentialIID: InstanceID, public signature: Buffer) {
    }
}

CredentialStruct.register();
Credential.register();
Attribute.register();
