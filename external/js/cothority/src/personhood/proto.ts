import { Point, PointFactory } from "@dedis/kyber";
import Long from "long";
import Moment from "moment";
import { Message, Properties } from "protobufjs/light";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";

export class PopPartyStruct extends Message<PopPartyStruct> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.PopPartyStruct", PopPartyStruct, PopDesc, Attendees, LRSTag);
    }

    state: number;
    readonly organizers: number;
    readonly finalizations: string[];
    readonly description: PopDesc;
    readonly attendees: Attendees;
    readonly miners: LRSTag[];
    readonly miningReward: Long;
    readonly previous: Buffer;
    readonly next: Buffer;

    constructor(props?: Properties<PopPartyStruct>) {
        super(props);

        this.finalizations = this.finalizations || [];
        this.miners = this.miners || [];
        this.previous = Buffer.from(this.previous || EMPTY_BUFFER);
        this.next = Buffer.from(this.next || EMPTY_BUFFER);

        /* Protobuf aliases */

        Object.defineProperty(this, "miningreward", {
            get(): Long {
                return this.miningReward;
            },
            set(value: Long) {
                this.miningReward = value;
            },
        });
    }

    /**
     * Replace the current attendees by the new ones and sort them, so that different
     * organizers scanning in a different order get the same result.
     *
     * @param publics Public keys of the new attendees
     */
    updateAttendes(publics: Point[]): void {
        const keys = publics.map((p) => p.toProto());
        keys.sort((a, b) => Buffer.compare(a, b));
        this.attendees.keys.splice(0, this.attendees.keys.length, ...keys);
    }
}

export class FinalStatement extends Message<FinalStatement> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.FinalStatement", FinalStatement, PopDesc, Attendees);
    }

    readonly desc: PopDesc;
    readonly attendees: Attendees;
}

export class PopDesc extends Message<PopDesc> {

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
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.PopDesc", PopDesc);
    }

    readonly name: string;
    readonly purpose: string;
    readonly datetime: Long; // in seconds
    readonly location: string;

    constructor(props?: Properties<PopDesc>) {
        super(props);
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

    /**
     * Get the keys as kyber points
     * @returns a list of points
     */
    get publics(): Point[] {
        return this.keys.map((k) => PointFactory.fromProto(k));
    }
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.Attendees", Attendees);
    }

    readonly keys: Buffer[];

    constructor(properties?: Properties<Attendees>) {
        super(properties);

        this.keys = this.keys.slice() || [];
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
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.LRSTag", LRSTag);
    }

    readonly tag: Buffer;

    constructor(props?: Properties<LRSTag>) {
        super(props);

        this.tag = Buffer.from(this.tag || EMPTY_BUFFER);
    }
}

PopPartyStruct.register();
FinalStatement.register();
PopDesc.register();
Attendees.register();
LRSTag.register();
