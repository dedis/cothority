import Long from "long";
import { Message } from "protobufjs/light";
import { Roster } from "../network";
import { registerMessage } from "../protobuf";

export class Transaction extends Message<Transaction> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("Transaction", Transaction, Master, Link, Election, Ballot, Mix, Partial);
    }
    readonly master: Master | null;
    readonly link: Link | null;
    readonly election: Election | null;
    readonly ballot: Ballot | null;
    readonly mix: Mix | null;
    readonly partial: Partial | null;
}

export class Master extends Message<Master> {

    static register() {
        registerMessage("Master", Master, Roster);
    }
    readonly id: Buffer;
    readonly roster: Roster;
    readonly admins: number[];
    readonly key: Buffer;
}

export class Link extends Message<Link> {

    static register() {
        registerMessage("Link", Link);
    }
    readonly id: Buffer;
}

export class Election extends Message<Election> {

    static register() {
        registerMessage("Election", Election, Footer);
    }
    readonly name: { [k: string]: string };
    readonly creator: number;
    readonly users: number[];
    readonly id: Buffer;
    readonly master: Buffer;
    readonly roster: Roster;
    readonly key: Buffer;
    readonly masterkey: Buffer;
    readonly stage: number;
    readonly candidates: number[];
    readonly maxchoices: number;
    readonly subtitle: { [k: string]: string };
    readonly moreinfo: string;
    readonly start: Long;
    readonly end: Long;
    readonly theme: string;
    readonly footer: Footer;
    readonly voted: Buffer;
    readonly moreinfolang: string[][];
}

export class Footer extends Message<Footer> {

    static register() {
        registerMessage("Footer", Footer);
    }
    readonly text: string;
    readonly contacttitle: string;
    readonly contactphone: string;
    readonly contactemail: string;
}

export class Ballot extends Message<Ballot> {

    static register() {
        registerMessage("Ballot", Ballot);
    }
    readonly user: number;
    readonly alpha: Buffer;
    readonly beta: Buffer;
}

export class Mix extends Message<Mix> {

    static register() {
        registerMessage("Mix", Mix, Ballot);
    }
    readonly ballots: Ballot[];
    readonly proof: Buffer;
    readonly nodeid: Buffer;
    readonly signature: Buffer;
}

export class Partial extends Message<Partial> {

    static register() {
        registerMessage("Partial", Partial);
    }
    readonly points: Buffer[];
    readonly nodeid: Buffer;
    readonly signature: Buffer;
}

Transaction.register();
Master.register();
Link.register();
Election.register();
Footer.register();
Ballot.register();
Mix.register();
Partial.register();
