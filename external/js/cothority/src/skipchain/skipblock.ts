import { Message } from 'protobufjs';
import { Roster } from '../network/proto';
import { registerMessage } from '../protobuf';
import { createHash } from 'crypto';

export class SkipBlock extends Message<SkipBlock> {
    readonly hash: Buffer;
    readonly roster: Roster;
    readonly index: number;
    readonly maxHeight: number;
    readonly baseHeight: number;
    readonly data: Buffer;
    readonly forward: ForwardLink[];

    get forwardLinks(): ForwardLink[] {
        return this.forward;
    }
}

export class ForwardLink extends Message<ForwardLink> {
    readonly from: Buffer;
    readonly to: Buffer;
    readonly newRoster: Roster;
    readonly signature: ByzcoinSignature;

    verify(): Error {
        const h = createHash('sha256');
        h.update(this.from);
        h.update(this.to);

        if (!h.digest().equals(this.signature.msg)) {
            return new Error('recreated message does not match');
        }

        // TODO: BLS signature verification

        return null;
    }
}

export class ByzcoinSignature extends Message<ByzcoinSignature> {
    readonly msg: Buffer;
    readonly sig: Buffer;
}

registerMessage('SkipBlock', SkipBlock);
registerMessage('ForwardLink', ForwardLink);
registerMessage('ByzcoinSig', ByzcoinSignature);
