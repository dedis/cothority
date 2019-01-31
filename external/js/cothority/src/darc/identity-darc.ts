import { Message } from "protobufjs";
import Identity from "./identity";
import IdentityWrapper from "./identity-wrapper";

export default class IdentityDarc extends Message<IdentityDarc> implements Identity {
    readonly id: Buffer;

    verify(msg: Buffer, signature: Buffer): boolean {
        return false;
    }

    typeString(): string {
        return 'darc';
    }

    toWrapper(): IdentityWrapper {
        return new IdentityWrapper({ darc: this });
    }

    toBytes(): Buffer {
        return this.id;
    }

    toString(): string {
        return `${this.typeString()}:${this.id.toString('hex')}`
    }
}
