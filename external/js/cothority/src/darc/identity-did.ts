import { Message, Properties } from "protobufjs/light";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";
import IdentityWrapper, { IIdentity } from "./identity-wrapper";

/**
 * Identity based on a DID
 */
export default class IdentityDid extends Message<IdentityDid> implements IIdentity {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("IdentityDID", IdentityDid);
    }

    readonly method: Buffer;
    readonly did: Buffer;

    constructor(props?: Properties<IdentityDid>) {
        super(props);
        this.method = Buffer.from(this.method || EMPTY_BUFFER);
        this.did = Buffer.from(this.did || EMPTY_BUFFER);
    }

    /** @inheritdoc */
    verify(msg: Buffer, signature: Buffer): boolean {
        return false;
    }

    /** @inheritdoc */
    toBytes(): Buffer {
        return Buffer.from(`did:${this.method}:${this.did}`);
    }

    /** @inheritdoc */
    toString(): string {
        return `did:${this.method}:${this.did}`;
    }
}
