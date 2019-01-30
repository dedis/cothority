import { Message } from "protobufjs";
import Instance, { InstanceID } from "../instance";
import ByzCoinRPC from "../byzcoin-rpc";
import Signer from "../../darc/signer";
import ClientTransaction, { Instruction, Argument } from "../client-transaction";
import Proof from "../proof";
import { registerMessage } from "../../protobuf";

export default class CredentialInstance {
    static readonly contractID = "credential";

    private rpc: ByzCoinRPC;
    private instance: Instance;
    private credential: CredentialStruct;

    constructor(bc: ByzCoinRPC, inst: Instance) {
        this.rpc = bc;
        this.instance = inst;
        this.credential = CredentialStruct.decode(inst.data);
    }

    get darcID(): InstanceID {
        return this.instance.darcID;
    }

    async update(): Promise<CredentialInstance> {
        const proof = await this.rpc.getProof(this.instance.id);
        if (!proof.matches()) {
            throw new Error('fail to get a matching proof');
        }

        this.instance = Instance.fromProof(proof);
        this.credential = CredentialStruct.decode(this.instance.data);
        return this;
    }

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
            CredentialInstance.contractID,
            "update",
            [new Argument({ name: "credential", value: this.credential.toBytes() })],
        );
        await instr.updateCounters(this.rpc, [owner]);

        const ctx = new ClientTransaction({ instructions: [instr] });
        ctx.signWith([owner]);

        await this.rpc.sendTransactionAndWait(ctx);
        return this;
    }

    static async fromProof(bc: ByzCoinRPC, p: Proof): Promise<CredentialInstance> {
        if (!p.matches()) {
            throw new Error('fail to get a matching proof');
        }

        return new CredentialInstance(bc, Instance.fromProof(p));
    }

    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<CredentialInstance> {
        return CredentialInstance.fromProof(bc, await bc.getProof(iid));
    }
}

export class CredentialStruct extends Message<CredentialStruct> {
    readonly credentials: Credential[];

    toBytes(): Buffer {
        return Buffer.from(CredentialStruct.encode(this).finish());
    }
}

export class Credential extends Message<Credential> {
    readonly name: string;
    readonly attributes: Attribute[];
}

export class Attribute extends Message<Attribute> {
    readonly name: string;
    readonly value: Buffer;
}

registerMessage('personhood.CredentialStruct', CredentialStruct);
registerMessage('personhood.Credential', Credential);
registerMessage('personhood.Attribute', Attribute);
