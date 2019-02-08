import { Point, PointFactory } from "@dedis/kyber";
import Moment from "moment";
import { Message, Properties } from "protobufjs";
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

    /**
     * Replace the current attendees by the new ones
     *
     * @param publics Public keys of the new attendees
     */
    updateAttendes(publics: Point[]): void {
        const keys = publics.map((p) => p.toProto());
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

    /**
     * Getter for the timestamp
     * @returns the timestamp as a number
     */
    get timestamp(): number {
        return this.datetime.toNumber();
    }

    /**
     * Format the timestamp into a human readable string
     * @returns a string of the time
     */
    get dateString(): string {
        return new Date(this.timestamp).toString().replace(/ GMT.*/, "");
    }

    /**
     * Format the timestamp to a unique string
     * @returns the string
     */
    get uniqueName(): string {
        const d = new Date(this.timestamp);
        return Moment(d).format("YY-MM-DD HH:mm");
    }

    /**
     * Helper to encode the statement using protobuf
     * @returns the bytes
     */
    toBytes(): Buffer {
        return Buffer.from(PopDesc.encode(this).finish());
    }
}

export class Attendees extends Message<Attendees> {
    readonly keys: Buffer[];

    constructor(properties?: Properties<Attendees>) {
        super(properties);

        if (!properties || !properties.keys) {
            this.keys = [];
        }
    }

    /**
     * Get the keys as kyber points
     * @returns a list of points
     */
    get publics(): Point[] {
        return this.keys.map((k) => PointFactory.fromProto(k));
    }

    /**
     * Helper to encode the attendees using protobuf
     * @returns the bytes
     */
    toBytes(): Buffer {
        return Buffer.from(Attendees.encode(this).finish());
    }
}

export class LRSTag extends Message<LRSTag> {
    readonly tag: Buffer;
}

/* TODO: remove after personhood.online is merged
registerMessage("personhood.PopPartyStruct", PopPartyStruct);
registerMessage("personhood.FinalStatement", FinalStatement);
registerMessage("personhood.PopDesc", PopDesc);
registerMessage("personhood.Attendees", Attendees);
registerMessage("personhood.LRSTag", LRSTag);
*/
