import Moment from 'moment';
import { Message } from "protobufjs";
import { registerMessage } from "../../../protobuf";

export class PopPartyStruct extends Message<PopPartyStruct> {
    readonly state: number;
    readonly organizers: number;
    readonly finalizations: string[];
    readonly description: PopDesc;
    readonly attendees: Attendees;
    readonly miners: LRSTag[];
    readonly miningreward: Long;
    readonly previous: Buffer;
    readonly next: Buffer;

    updateAttendes(keys: Buffer[]): void {
        this.attendees.keys.splice(0, this.attendees.keys.length, ...keys);
    }
}

export class FinalStatement extends Message<FinalStatement> {
    readonly desc: PopDesc;
    readonly attendees: Attendees;
}

export class PopDesc extends Message<PopDesc> {
    readonly name: string;
    readonly purpose: string;
    readonly datetime: Long; // in seconds
    readonly location: string;

    get timestamp(): number {
        return this.datetime.toNumber();
    }

    get dateString(): string {
        return new Date(this.timestamp).toString().replace(/ GMT.*/, "");
    }

    get uniqueName(): string {
        const d = new Date(this.timestamp);
        return Moment(d).format('YY-MM-DD HH:mm');
    }

    toBytes(): Buffer {
        return Buffer.from(PopDesc.encode(this).finish());
    }
}

export class Attendees extends Message<Attendees> {
    readonly keys: Buffer[];

    toBytes(): Buffer {
        return Buffer.from(Attendees.encode(this).finish());
    }
}

export class LRSTag extends Message<LRSTag> {
    readonly tag: Buffer;
}

registerMessage('personhood.PopPartyStruct', PopPartyStruct);
registerMessage('personhood.FinalStatement', FinalStatement);
registerMessage('personhood.PopDesc', PopDesc);
registerMessage('personhood.Attendees', Attendees);
registerMessage('personhood.LRSTag', LRSTag);
